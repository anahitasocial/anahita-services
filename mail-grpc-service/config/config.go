package config

import (
	an_constants "anahita-libs/constants"
	"errors"
	"log"
	"os"
	"strings"
)

const (
	ErrMissingSmtpHost       = "SMTP host is missing"
	ErrMissingSmtpPort       = "SMTP port is missing"
	ErrMissingDeveloperEmail = "Developer email is missing, and is required in development"
	ErrInvalidSmtpSecure     = `SMTP_SECURE must be true or false ` +
		`(true = implicit TLS, usually port 465; false = STARTTLS, usually port 587). ` +
		`Values such as "ssl" or "tls" are not accepted, because guessing at them ` +
		`would silently pick a transport security mode`
)

type Smtp struct {
	// SmtpUser and SmtpPassword are OPTIONAL. An internal relay or a local
	// mail sink that accepts unauthenticated mail from inside the network is
	// an ordinary arrangement, and requiring credentials would exclude it.
	// Empty user means the provider skips AUTH entirely.
	SmtpUser     string
	SmtpPassword string
	SmtpHost     string
	SmtpPort     int

	// SmtpSecure selects implicit TLS (port 465) over STARTTLS (587).
	SmtpSecure bool

	// smtpSecureInvalid records that SMTP_SECURE was set to something that
	// could not be parsed. Validate turns that into a refusal to start — see
	// ErrInvalidSmtpSecure.
	//
	// Phrased as the NEGATIVE deliberately, so the zero value means "fine".
	// A "valid" flag would make every Config built as a struct literal — every
	// test, and any future caller not going through NewConfig — fail validation
	// on a field it never heard of.
	smtpSecureInvalid bool
}

type Config struct {
	Smtp Smtp
	Env  string

	// DeveloperEmail is where every outbound message goes in development.
	//
	// Without it, testing any flow that sends mail means either sending real
	// mail to real addresses from a laptop, or not testing it. Several flows in
	// auth-service — password reset, email change, account deletion — cannot be
	// exercised at all without receiving the message, since the link in it is
	// the next step.
	//
	// Required only in development; see Validate.
	DeveloperEmail string

	// SiteName is what this installation calls itself, shown in the header of
	// every message and used as the logo's alt text.
	//
	// Configured HERE rather than passed in by each caller, because it is a
	// property of the installation's mail rather than of whichever service
	// happened to send it — a second service sending mail should not have to
	// remember to brand it, and should not be able to brand it differently.
	SiteName string

	// SiteLogoURL is an absolute, publicly reachable URL for the logo.
	//
	// A URL rather than an embedded image, deliberately:
	//
	//   - A base64 data: URI is the option that sounds best and works worst.
	//     Gmail strips them outright and Outlook blocks them, so the logo
	//     would be missing in most inboxes.
	//   - A CID attachment (multipart/related) does render, but it puts an
	//     attachment indicator on the message. On "your password was changed"
	//     an attachment reads as phishing, which is the opposite of what a
	//     logo is here to do.
	//
	// A remote URL is what every large transactional sender uses. Some clients
	// block remote images until asked, so the template must read correctly
	// without it — see message_action.html.tmpl, where the site name is text
	// beside the image rather than inside it.
	//
	// Empty means no image at all, which is the safe default: a broken image
	// is worse branding than none. In production this usually points at the
	// auth host's own asset, e.g. https://auth.example.com/static/images/logo.png
	SiteLogoURL string
}

func NewConfig() *Config {
	smtpSecure, smtpSecureValid := parseBoolEnv("SMTP_SECURE", false)

	config := &Config{
		Env:            getEnv("ENV", an_constants.ConfigEnvDev),
		DeveloperEmail: os.Getenv("DEVELOPER_EMAIL"),
		SiteName:       os.Getenv("SITE_NAME"),
		SiteLogoURL:    os.Getenv("SITE_LOGO_URL"),
		Smtp: Smtp{
			SmtpUser:     os.Getenv("SMTP_USER"),
			SmtpPassword: os.Getenv("SMTP_PASSWORD"),
			SmtpHost:     os.Getenv("SMTP_HOST"),
			SmtpPort:     getIntEnv("SMTP_PORT", 587),
			SmtpSecure:   smtpSecure,
			// Carried so Validate can refuse an unparseable value rather than
			// quietly running with the default.
			smtpSecureInvalid: !smtpSecureValid,
		},
	}

	return config
}

// Validate checks the configuration the service actually needs.
//
// SMTP is the only transport, so there is no provider to switch on any more.
// Host and port are required; user and password are not — see Smtp.
func (config *Config) Validate() error {
	var missingFields []string

	if config.Smtp.SmtpHost == "" {
		missingFields = append(missingFields, ErrMissingSmtpHost)
	}
	if config.Smtp.SmtpPort == 0 {
		missingFields = append(missingFields, ErrMissingSmtpPort)
	}
	// Refused rather than defaulted. This selects whether the session is
	// encrypted from the first byte or upgraded in place, and a value nobody
	// can parse should stop the service rather than pick one.
	if config.Smtp.smtpSecureInvalid {
		missingFields = append(missingFields, ErrInvalidSmtpSecure)
	}

	// Not fatal, because a server may legitimately listen anywhere — but these
	// two pairings are almost always a mistake, and the symptom is an opaque
	// handshake timeout rather than anything naming the cause.
	if config.Smtp.SmtpSecure && config.Smtp.SmtpPort == 587 {
		log.Println("mail: SMTP_SECURE=true with port 587 — 587 normally expects " +
			"STARTTLS (SMTP_SECURE=false); implicit TLS normally uses 465")
	}
	if !config.Smtp.SmtpSecure && config.Smtp.SmtpPort == 465 {
		log.Println("mail: SMTP_SECURE=false with port 465 — 465 normally expects " +
			"implicit TLS (SMTP_SECURE=true); STARTTLS normally uses 587")
	}

	// Required in development only.
	//
	// The redirect is what stops a laptop mailing real people, so booting
	// without it in development is the one case worth refusing outright — an
	// empty override would silently blank the recipient list instead. Requiring
	// it everywhere, as protect-services does, only forces production
	// deployments to carry a value nothing reads.
	if config.IsDev() && config.DeveloperEmail == "" {
		missingFields = append(missingFields, ErrMissingDeveloperEmail)
	}

	if len(missingFields) > 0 {
		return errors.New("missing fields: " + strings.Join(missingFields, ", "))
	}

	return nil
}

func (config *Config) MustValidate() {
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
}

func (config *Config) IsDev() bool {
	return config.Env == an_constants.ConfigEnvDev
}

func (config *Config) IsStaging() bool {
	return config.Env == an_constants.ConfigEnvStaging
}

func (config *Config) IsProd() bool {
	return config.Env == an_constants.ConfigEnvProd
}

// GetSmtpUser returns the SMTP user
func (config *Config) GetSmtpUser() string {
	return config.Smtp.SmtpUser
}

// GetSmtpPassword returns the SMTP password
func (config *Config) GetSmtpPassword() string {
	return config.Smtp.SmtpPassword
}

// GetSmtpHost returns the SMTP host
func (config *Config) GetSmtpHost() string {
	return config.Smtp.SmtpHost
}

// GetSmtpPort returns the SMTP port
func (config *Config) GetSmtpPort() int {
	return config.Smtp.SmtpPort
}

// GetSmtpSecure returns whether SMTP is secure
func (config *Config) GetSmtpSecure() bool {
	return config.Smtp.SmtpSecure
}

// GetSiteName returns the installation's name, or a neutral fallback.
//
// Falls back rather than rendering an empty header — the alternative is a
// message with a blank space where the brand should be, which looks broken
// rather than unbranded.
func (config *Config) GetSiteName() string {
	if config.SiteName != "" {
		return config.SiteName
	}

	return "Notifications"
}

// GetSiteLogoURL returns the logo URL, or empty when none is configured.
//
// Empty is a supported state, not a misconfiguration: the template renders the
// site name as text instead. Anything that is not an absolute http(s) URL is
// treated as absent, because a relative path in an email resolves against
// nothing and renders as a broken image in every client.
func (config *Config) GetSiteLogoURL() string {
	if strings.HasPrefix(config.SiteLogoURL, "http://") ||
		strings.HasPrefix(config.SiteLogoURL, "https://") {
		return config.SiteLogoURL
	}

	if config.SiteLogoURL != "" {
		log.Printf("mail: ignoring SITE_LOGO_URL %q — an email logo needs an "+
			"absolute http(s) URL, since a relative path resolves against "+
			"nothing in a mail client", config.SiteLogoURL)
	}

	return ""
}

// GetDeveloperEmail returns the address every message is redirected to in
// development. Empty outside development, where nothing reads it.
func (config *Config) GetDeveloperEmail() string {
	return config.DeveloperEmail
}
