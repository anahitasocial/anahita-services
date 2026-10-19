package services

import (
	"anahita-libs/about"
	"anahita-libs/policy"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	libs_constants "anahita-libs/constants"
	libs_entities "anahita-libs/data/entities"
	libs_services "anahita-libs/data/services"

	"auth/constants"
	"auth/data/entities"
	"auth/data/repositories"
)

// ============================================================================
// SeedSuperAdmin — bootstrap the first super administrator
// ============================================================================
//
// Nothing in the Go stack ever assigned the super-administrator authority
// level. It appears only in comparisons — nine of them — and never on the left
// of an assignment, so a fresh install from schema/init.sql had no way to
// produce one at all. The super administrator in the reference database came
// from the legacy PHP migration; a new deployment had nobody who could invite
// anybody, and no route to fix it except hand-written SQL.
//
// This closes that. It runs at boot, does nothing when the installation
// already has an owner, and is the one place person_type is written as
// anything other than "registered".

// SuperAdminBootstrapTTL is how long the set-password link stays live.
//
// Far longer than the fifteen minutes an ordinary reset gets, because the two
// are not the same event. A reset is requested by somebody sitting at the
// form; this one is minted by a container starting up, and the operator may
// not read the logs or the mailbox for hours. A link that expired before it
// was ever seen is a dead end, and the account it guards cannot be signed into
// in the meantime — the seeded password hash is empty, which
// TestEmptyPasswordHashIsUnusable pins as unusable.
const SuperAdminBootstrapTTL = 48 * time.Hour

// seedAliasStrip removes everything that is not a handle character.
var seedAliasStrip = regexp.MustCompile(`[^a-z0-9_-]+`)

// SuperAdminSeedEnv is what the seed needs from the service's wiring.
type SuperAdminSeedEnv struct {
	Repos  *repositories.Repositories
	Actors libs_services.ActorsGRPCInterface
	Config SuperAdminSeedConfig

	// SendLink delivers the set-password link by email. Optional: when it is
	// nil, or when it fails, the link still reaches the log. See below for why
	// that is the important half.
	SendLink func(user *entities.User, token *entities.Token) error
}

// SuperAdminSeedConfig is the configuration the seed reads.
type SuperAdminSeedConfig interface {
	GetSuperAdminEmail() string
	GetSuperAdminUsername() string
	GetSuperAdminName() string
	GetAuthURL() string
}

// SeedSuperAdmin creates the first super administrator if the installation has
// none, and issues the link that lets the operator set its password.
//
// Reports whether it created the account, so the caller does not then re-issue
// the link it has just issued — two links in one boot log, the first of them
// already retired by the second, is a worse message than one.
//
// IDEMPOTENT AND SKIP-IF-EXISTS, for the same reason the OAuth client seed is:
// this runs on every boot, and a redeploy must never re-create or overwrite.
// Two separate guards, because they answer different questions —
// CountSuperAdmins asks whether anybody owns this installation, and the email
// lookup asks whether this particular account was already made.
//
// Called from main.go after Validate. A failure here is logged and does not
// stop the service: an installation that cannot seed an administrator is still
// an installation its existing members can sign in to, and refusing to start
// would turn a bootstrap problem into an outage.
func SeedSuperAdmin(env *SuperAdminSeedEnv) (bool, error) {
	email := strings.ToLower(strings.TrimSpace(env.Config.GetSuperAdminEmail()))
	if email == "" {
		return false, nil
	}

	// Guard one: does this installation already have an owner?
	count, err := env.Repos.Users.CountSuperAdmins()
	if err != nil {
		return false, fmt.Errorf("count super administrators: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	// Guard two: is this address already an account?
	//
	// Reached when SUPER_ADMIN_EMAIL names somebody who signed up ordinarily,
	// or when a previous boot seeded the account and it was then demoted or
	// disabled. Promoting whoever holds the address would be the wrong move in
	// both cases — an env var is not consent — so it stops and says so.
	if existing, err := env.Repos.Users.ReadByEmail(email); err == nil && existing != nil {
		log.Printf("super admin seed: %s already has an account (person=%d) but the "+
			"installation has no active super administrator; refusing to promote it "+
			"automatically", email, existing.ID)
		return false, nil
	}

	alias, err := availableSeedAlias(env, email)
	if err != nil {
		return false, err
	}

	// The actor node first, through the graph — as signup and invite-accept
	// do, and for the same reasons: the graph applies the type's default
	// features, enforces the handle namespace, and publishes the event search
	// and feed consume.
	actor := libs_entities.NewAnahitaNode()
	actor.Type = libs_constants.NodeActorPersonEntityType
	actor.Service = libs_constants.PersonService
	actor.Alias = alias
	actor.Name = env.Config.GetSuperAdminName()
	// Public, unless the installation allows nothing new to be: then a new
	// profile is for signed-in people, like everything else created here.
	actor.Access = policy.MinContentAccess()
	actor.PersonType = libs_constants.PersonTypeSuperAdmin
	actor.Enabled = true
	actor.CreatedAt = time.Now()
	actor.UpdatedAt = time.Now()

	created, err := env.Actors.Add(actor, libs_entities.NewViewer())
	if err != nil {
		return false, fmt.Errorf("create the super administrator actor: %w", err)
	}

	user := entities.NewUser()
	user.ID = created.ID
	user.Alias = created.Alias
	user.Name = created.Name
	user.Email = email
	user.PersonType = libs_constants.PersonTypeSuperAdmin
	user.Enabled = true
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	// Verified. The operator set SUPER_ADMIN_EMAIL themselves and claims the
	// account through a link sent to it; requiring a separate confirmation
	// would put one more thing between a fresh installation and having an
	// administrator, which is the failure this seed exists to prevent.
	user.MarkEmailVerified()
	// Password deliberately left empty. An empty bcrypt hash cannot be
	// satisfied by any input, including an empty one, so there is nothing to
	// guess in the window between this row existing and the operator claiming
	// it. The set-password link below is the only way in.
	//
	// Consent is left empty too, and honestly so: nobody has agreed to
	// anything yet. It is collected on the set-password page, where a real
	// person with a real IP address ticks a real box.

	if _, err := env.Repos.Users.Add(user); err != nil {
		// Compensating purge, as in signup: two writes across two services
		// with no transaction between them.
		//
		// It is asked for, not waited for: the graph queues it. And it is
		// refused when this actor is the only super administrator there is,
		// which on a first start it is. Either way the orphan is named in the
		// log, which is all that can be done from here.
		result, purgeErr := env.Actors.RequestPurge(&libs_services.ActorPurgeRequest{
			IDs:                 []int64{created.ID},
			Type:                libs_constants.NodeActorPersonEntityType,
			AllowAdministrators: true,
		}, libs_entities.NewViewer())
		if purgeErr != nil || len(result.Refused) > 0 {
			log.Printf("super admin seed: orphaned actor %d: %v (purge failed or was refused: %v)",
				created.ID, err, purgeErr)
		}
		return false, fmt.Errorf("create the super administrator account: %w", err)
	}

	log.Printf("super admin seed: created %s (person=%d, handle=%s)", email, user.ID, user.Alias)

	return true, IssueSuperAdminBootstrapLink(env, user)
}

// IssueSuperAdminBootstrapLink mints a set-password token and delivers it two
// ways: by email, and by printing the full URL to the log.
//
// THE LOG LINE IS THE GUARANTEE, not a convenience. On a fresh deployment SMTP
// is routinely the last thing configured and the first thing misconfigured. If
// email were the only path, a wrong SMTP_HOST would lock the operator out of a
// brand-new installation with no recovery except hand-written SQL — and the
// symptom would be silence, which reads as "the seed did not run".
//
// Exported because main.go re-issues on every boot while the seeded account
// still has no password, so a link that expired unread is never a dead end.
func IssueSuperAdminBootstrapLink(env *SuperAdminSeedEnv, user *entities.User) error {
	// Retire any earlier link first. Re-issuing on every boot would otherwise
	// leave one live token per restart, each valid for two days — a set of
	// keys to the installation's most privileged account, growing with the
	// uptime of the deployment.
	if _, err := env.Repos.Tokens.InvalidateOutstanding(
		constants.TokenTypePasswordReset, user.Email,
	); err != nil {
		log.Println("super admin seed: retiring earlier links failed:", err)
	}

	token := entities.NewToken()
	token.Type = constants.TokenTypePasswordReset
	token.Status = constants.TokenStatusPending
	token.RecipientEmail = user.Email
	token.ExpiredAfter = int(SuperAdminBootstrapTTL.Seconds())
	token.IsUsed = false
	token.CreatedAt = time.Now()
	token.CreatedBy = user.ID
	token.UpdatedAt = time.Now()
	token.UpdatedBy = user.ID

	if err := token.GenerateToken(); err != nil {
		return fmt.Errorf("generate the bootstrap token: %w", err)
	}

	if _, err := env.Repos.Tokens.Add(token); err != nil {
		return fmt.Errorf("store the bootstrap token: %w", err)
	}

	url := fmt.Sprintf("%s/password/reset?token=%s",
		strings.TrimRight(env.Config.GetAuthURL(), "/"), token.Body)

	// Printed before the email is attempted, so it is on record even if the
	// send panics or hangs.
	log.Printf("super admin seed: set the password for %s within %s:\n  %s",
		user.Email, SuperAdminBootstrapTTL, url)

	// And the pledge Anahita's creator asks of whoever installs it (roadmap
	// row 102): said here, and ticked on the page the link opens.
	lines := make([]string, 0, len(about.PledgeItems))
	for _, item := range about.PledgeItems {
		lines = append(lines, "  - "+item.In("en-CA"))
	}
	log.Printf("super admin seed: %s\n%s\n%s",
		about.PledgeIntro.In("en-CA"), about.PledgeLead.In("en-CA"), strings.Join(lines, "\n"))

	if env.SendLink == nil {
		return nil
	}

	if err := env.SendLink(user, token); err != nil {
		// Not returned. The link is already in the log, which is the path that
		// works when mail does not — so a failed send is worth saying loudly
		// and is not a failed bootstrap.
		log.Println("super admin seed: emailing the link failed; use the URL above:", err)
	}

	return nil
}

// NeedsSuperAdminBootstrapLink reports whether the seeded account is still
// unclaimed, so main.go can re-issue.
//
// An empty password is the signal, and it is a reliable one: nothing but this
// seed creates an account without a password, and the only way to fill one in
// is to use the link.
func NeedsSuperAdminBootstrapLink(env *SuperAdminSeedEnv) (*entities.User, bool) {
	email := strings.ToLower(strings.TrimSpace(env.Config.GetSuperAdminEmail()))
	if email == "" {
		return nil, false
	}

	user, err := env.Repos.Users.ReadByEmail(email)
	if err != nil || user == nil {
		return nil, false
	}

	return user, user.Password == "" && user.IsSuperAdmin()
}

// availableSeedAlias picks a handle for the seeded account.
//
// SUPER_ADMIN_USERNAME wins when set. Otherwise it is the email local part,
// lowercased and stripped — and unlike invite-accept, this one DOES suffix on
// collision. Invite-accept can refuse and let a person choose; a container
// starting up has nobody to ask, and failing the bootstrap because "admin" is
// taken would leave the installation with no owner over a name.
func availableSeedAlias(env *SuperAdminSeedEnv, email string) (string, error) {
	base := strings.ToLower(strings.TrimSpace(env.Config.GetSuperAdminUsername()))
	if base == "" {
		local, _, _ := strings.Cut(email, "@")
		base = strings.ToLower(local)
	}
	base = seedAliasStrip.ReplaceAllString(base, "")

	if base == "" {
		base = "admin"
	}

	// Three characters is the minimum a handle is allowed elsewhere.
	for len(base) < 3 {
		base += "0"
	}

	candidate := base
	for suffix := 2; suffix < 100; suffix++ {
		taken, err := env.Repos.Users.IsUsername(candidate)
		if err != nil {
			return "", fmt.Errorf("check the handle %q: %w", candidate, err)
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s%d", base, suffix)
	}

	return "", errors.New("could not find a free handle for the super administrator")
}
