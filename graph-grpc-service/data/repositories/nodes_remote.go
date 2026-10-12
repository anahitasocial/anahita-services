package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/site"

	sq "github.com/Masterminds/squirrel"
)

// What comes from another server: a person followed on Mastodon or Bluesky,
// and what they post.
//
// They are kept as nodes like any other, so that a feed, a follow and a
// block need to know nothing new. What makes one of them not ours is its
// URI, what it is called where it lives, and that is all this file is
// about: a node with a URI is only ever written here, and nothing here ever
// writes a node without one.
//
// THE SERVICES THAT CALL THIS READ THE INTERNET. A fault in one of them, or
// something sent to one by somebody who means harm, must not be able to
// reach what was made here. So what may be kept is narrow, and it is
// narrowed here and not left to the caller:
//
//   - only a person of another server, or a note;
//   - a note only on the profile of a person of another server, and by
//     them: nothing arrives as written by somebody here, or on a profile
//     here;
//   - only under an address that is not one of this installation's own;
//   - a node that is known is brought up to date in its words and nothing
//     else. What kind of thing it is, whose it is and what it is called
//     there never change.

const (
	// The longest URI kept. The column's width.
	remoteURIMax = 512
	// What a remote person or post is open to: everybody who may read the
	// site. Only what is public elsewhere is ever fetched.
	remoteAccess = "public"
)

var (
	ErrRemoteURI      = errors.New("what comes from another server needs the address it has there: https or at, and not one of ours")
	ErrRemoteDomain   = errors.New("what comes from another server needs the name of that server")
	ErrRemoteType     = errors.New("only a person of another server, or a note, is kept from one")
	ErrRemoteAlias    = errors.New("a person of another server is called by their name and their server's")
	ErrRemoteOwner    = errors.New("a note from another server is kept on the profile of a person of another server")
	ErrRemoteMismatch = errors.New("what is known by that address is something else")
)

// remoteActorTypes is the kinds of person another server has.
var remoteActorTypes = []string{
	constants.NodeActorActivityPubPersonEntityType,
	constants.NodeActorATProtoPersonEntityType,
}

// IsRemoteActorType says whether a type is a person of another server.
func IsRemoteActorType(nodeType string) bool {
	for _, remote := range remoteActorTypes {
		if nodeType == remote {
			return true
		}
	}
	return false
}

// ReadByURI retrieves a single node by what it is called on the server it
// came from. Nothing made here is found by it.
//
// No access rule, as with Read. The caller decides who is asking.
func (r *nodesRepo) ReadByURI(uri string) (*entities.AnahitaNode, error) {
	if uri == "" {
		return nil, nil
	}
	query := GetBaseSelectQuery().Where(sq.Eq{"node.uri": uri})
	return FetchNode(r.db, query)
}

const readRemoteByURI = `
SELECT id, type, COALESCE(owner_id, 0)
FROM nodes
WHERE uri = ?`

const readRemoteOwner = `
SELECT type, uri IS NOT NULL
FROM nodes
WHERE id = ?`

// UpsertRemote keeps a person or a note from another server: added if its
// URI is new, brought up to date if it is known. It answers with the node
// and whether it was new.
func (r *nodesRepo) UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error) {
	if err := r.validateRemote(node); err != nil {
		return nil, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	id, known, err := r.knownRemote(ctx, node)
	if err != nil {
		return nil, false, err
	}

	if !known {
		id, err = r.addRemote(ctx, node)
		// Two messages about one thing can arrive together. Whichever was
		// second finds the first one's row and brings it up to date.
		if isDuplicateOf(err, "uq_uri") {
			if id, known, err = r.knownRemote(ctx, node); err == nil && !known {
				err = ErrRemoteMismatch
			}
		}
		if err != nil {
			return nil, false, err
		}
	}

	if known {
		if err := r.editRemote(ctx, id, node); err != nil {
			return nil, false, err
		}
	}

	saved, err := r.Read(id)
	return saved, !known, err
}

const (
	readRemoteNote = `
SELECT id
FROM nodes
WHERE uri = ?
  AND owner_id = ?
  AND type = ?`

	deleteRemoteNoteEdges = `
DELETE FROM edges
WHERE node_a_id = ?
   OR node_b_id = ?`

	// "uri IS NOT NULL" again, here where the row goes: never something
	// that was made here, whatever the lines above found.
	deleteRemoteNote = `
DELETE FROM nodes
WHERE id = ?
  AND uri IS NOT NULL`
)

// DeleteRemote takes away a note that came from another server, because
// its author took it away there. It says whether there was one.
//
// Only a note, only one that came from elsewhere, and only when it is asked
// for in the name of whoever it belongs to: the service that asks is passing
// on what it was sent, and what it was sent may be a lie about whose a note
// is. Whatever points at the note (a like, a save) goes with it.
func (r *nodesRepo) DeleteRemote(uri string, ownerID int64) (bool, error) {
	if uri == "" || ownerID <= 0 {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	var id int64
	err := r.db.QueryRowContext(ctx, readRemoteNote, uri, ownerID, constants.NodeMediumNoteEntityType).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if _, err := r.db.ExecContext(ctx, deleteRemoteNoteEdges, id, id); err != nil {
		return false, err
	}
	result, err := r.db.ExecContext(ctx, deleteRemoteNote, id)
	if err != nil {
		return false, err
	}
	gone, err := result.RowsAffected()
	return gone == 1, err
}

const (
	readRemoteActor = `
SELECT id, type
FROM nodes
WHERE uri = ?`

	// Everybody here who followed them or was followed by them: whose
	// counts of followers and of people followed are about to be wrong.
	readRemoteActorNeighbours = `
SELECT DISTINCT IF(node_a_id = ?, node_b_id, node_a_id)
FROM edges
WHERE node_a_id = ?
   OR node_b_id = ?`

	deleteRemoteActorPostEdges = `
DELETE edge
FROM edges AS edge
JOIN nodes AS post
  ON post.id IN (edge.node_a_id, edge.node_b_id)
WHERE post.owner_id = ?
  AND post.uri IS NOT NULL`

	deleteRemoteActorPosts = `
DELETE FROM nodes
WHERE owner_id = ?
  AND uri IS NOT NULL`
)

// DeleteRemoteActor takes away somebody from another server, with what
// they posted and every follow, like and save that touches either. It says
// whether there was such a person, and who here was connected to them, for
// their counts to be put right.
//
// Only ever somebody who came from elsewhere: a person of another server,
// found by the address they have there. Nothing made here has one.
func (r *nodesRepo) DeleteRemoteActor(uri string) (bool, []int64, error) {
	if uri == "" {
		return false, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	var (
		id       int64
		nodeType string
	)
	err := r.db.QueryRowContext(ctx, readRemoteActor, uri).Scan(&id, &nodeType)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !IsRemoteActorType(nodeType)) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	rows, err := r.db.QueryContext(ctx, readRemoteActorNeighbours, id, id, id)
	if err != nil {
		return false, nil, err
	}
	var neighbours []int64
	for rows.Next() {
		var neighbour int64
		if err := rows.Scan(&neighbour); err != nil {
			rows.Close()
			return false, nil, err
		}
		neighbours = append(neighbours, neighbour)
	}
	if err := rows.Close(); err != nil {
		return false, nil, err
	}

	for _, statement := range []struct {
		query string
		args  []any
	}{
		{deleteRemoteActorPostEdges, []any{id}},
		{deleteRemoteActorPosts, []any{id}},
		{deleteRemoteNoteEdges, []any{id, id}},
		{deleteRemoteNote, []any{id}},
	} {
		if _, err := r.db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return false, nil, err
		}
	}

	return true, neighbours, nil
}

// knownRemote finds the node already kept under a URI, and refuses when it
// is not the thing now being sent.
func (r *nodesRepo) knownRemote(ctx context.Context, node *entities.AnahitaNode) (int64, bool, error) {
	var (
		id      int64
		was     string
		ownerID int64
	)
	err := r.db.QueryRowContext(ctx, readRemoteByURI, node.URI).Scan(&id, &was, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	// A person does not become a note, nor one protocol's person another's,
	// nor a note somebody else's.
	if was != node.Type || (!IsRemoteActorType(node.Type) && ownerID != node.OwnerID) {
		return 0, false, ErrRemoteMismatch
	}
	return id, true, nil
}

func (r *nodesRepo) addRemote(ctx context.Context, node *entities.AnahitaNode) (int64, error) {
	now := time.Now().UTC()
	created := node.CreatedAt
	// When it says it was written, unless it says nothing (a time that was
	// never set arrives as 1970) or says the future.
	if created.Unix() <= 0 || created.After(now) {
		created = now
	}

	columns := map[string]any{
		"type":          node.Type,
		"service":       serviceOf(node.Type),
		"uri":           node.URI,
		"origin_domain": node.OriginDomain,
		"name":          node.Name,
		"alias":         node.Alias,
		"body":          node.Body,
		"meta":          node.Meta,
		"language":      node.Language,
		"website_url":   node.WebsiteURL,
		"access":        remoteAccess,
		"enabled":       true,
		"starts_at":     created,
		"ends_at":       created,
		"created_at":    created,
		"updated_at":    now,
	}
	if node.AvatarFilename != "" {
		columns["avatar_filename"] = node.AvatarFilename
	}
	if !IsRemoteActorType(node.Type) {
		columns["owner_id"] = node.OwnerID
		columns["owner_type"] = node.OwnerType
		columns["created_by"] = node.OwnerID
		columns["modified_by"] = node.OwnerID
	}

	query, args, err := sq.Insert("nodes").SetMap(columns).ToSql()
	if err != nil {
		return 0, err
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isDuplicateOf(err, "uq_unique_alias") {
			return 0, fmt.Errorf("%w: %s is somebody already known", ErrRemoteAlias, node.Alias)
		}
		return 0, err
	}
	return result.LastInsertId()
}

// editRemote brings a known node's words up to date, and nothing else.
func (r *nodesRepo) editRemote(ctx context.Context, id int64, node *entities.AnahitaNode) error {
	columns := map[string]any{
		"name":        node.Name,
		"body":        node.Body,
		"meta":        node.Meta,
		"language":    node.Language,
		"website_url": node.WebsiteURL,
		"updated_at":  time.Now().UTC(),
	}
	// A person may come to be called something else where they are.
	if IsRemoteActorType(node.Type) {
		columns["alias"] = node.Alias
	}
	// The name their picture is stored under here, when a new one was
	// copied. Left as it is when nothing is said: bringing somebody's words
	// up to date does not take their picture away.
	if node.AvatarFilename != "" {
		columns["avatar_filename"] = node.AvatarFilename
	}

	query, args, err := sq.Update("nodes").SetMap(columns).
		Where(sq.Eq{"id": id}).
		// Belt and braces: never a node that was made here.
		Where(sq.NotEq{"uri": nil}).
		ToSql()
	if err != nil {
		return err
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		if isDuplicateOf(err, "uq_unique_alias") {
			return fmt.Errorf("%w: %s is somebody already known", ErrRemoteAlias, node.Alias)
		}
		return err
	}
	return nil
}

// validateRemote refuses what may not be kept from another server, and
// tidies what may.
func (r *nodesRepo) validateRemote(node *entities.AnahitaNode) error {
	if node == nil {
		return ErrRemoteType
	}

	node.URI = strings.TrimSpace(node.URI)
	if !validRemoteURI(node.URI) {
		return ErrRemoteURI
	}

	node.OriginDomain = strings.ToLower(strings.TrimSpace(node.OriginDomain))
	if node.OriginDomain == "" || len(node.OriginDomain) > 255 || strings.ContainsAny(node.OriginDomain, "/ @") {
		return ErrRemoteDomain
	}

	switch {
	case IsRemoteActorType(node.Type):
		// alice@mastodon.social, alice.bsky.social: a name no username here
		// can be, since those are letters, digits, hyphens and underscores.
		node.Alias = strings.ToLower(strings.TrimSpace(node.Alias))
		if node.Alias == "" || len(node.Alias) > 255 || !strings.ContainsAny(node.Alias, "@.") || strings.ContainsAny(node.Alias, " /") {
			return ErrRemoteAlias
		}
		node.OwnerID, node.OwnerType = 0, ""

	case node.Type == constants.NodeMediumNoteEntityType:
		if node.OwnerID <= 0 {
			return ErrRemoteOwner
		}
		ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
		defer cancel()

		var (
			ownerType string
			remote    bool
		)
		err := r.db.QueryRowContext(ctx, readRemoteOwner, node.OwnerID).Scan(&ownerType, &remote)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (!remote || !IsRemoteActorType(ownerType))) {
			return ErrRemoteOwner
		}
		if err != nil {
			return err
		}
		node.OwnerType = ownerType
		node.Alias = ""

	default:
		return ErrRemoteType
	}

	return nil
}

// validRemoteURI says whether a URI is one that something elsewhere could
// have: an https address on a host that is not ours, or an at:// URI.
func validRemoteURI(uri string) bool {
	if uri == "" || len(uri) > remoteURIMax {
		return false
	}
	for _, c := range uri {
		if c <= ' ' || c > '~' {
			return false
		}
	}

	// An at:// URI names its owner by a DID, did:plc:…, whose colons are no
	// part of an address as the web has them. It is not read as one.
	if rest, ok := strings.CutPrefix(uri, "at://"); ok {
		return rest != ""
	}

	parsed, err := url.Parse(uri)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		return false
	}
	return !site.FromEnv().IsOurs(uri)
}

// serviceOf is the service named in a type: node.<family>.<service>.<kind>.
func serviceOf(nodeType string) string {
	parts := strings.Split(nodeType, ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

// isDuplicateOf says whether an error is MySQL refusing a second row with
// the same value under the named unique key.
func isDuplicateOf(err error, key string) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062") && strings.Contains(err.Error(), key)
}
