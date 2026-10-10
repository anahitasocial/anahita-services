package repositories

import (
	"anahita-libs/data/entities"
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// Edit updates an existing node based on the provided edit request and viewer context.
func (r *nodesRepo) Edit(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.dbTimeout)
	defer cancel()

	if err := r.validateEdit(node); err != nil {
		return nil, err
	}

	// Ensure StartsAt and EndsAt have default values if zero value
	if node.StartsAt.IsZero() {
		node.StartsAt = time.Now()
	}
	if node.EndsAt.IsZero() {
		// Default end date: same as start date (adjust if different logic needed)
		node.EndsAt = node.StartsAt
	}

	query := sq.Update("nodes").Where(sq.Eq{"id": node.ID})
	if node.Name != "" {
		query = query.Set("name", node.Name)
	}
	if node.Alias != "" {
		query = query.Set("alias", node.Alias)
	}
	if node.Body != "" {
		query = query.Set("body", node.Body)
	}
	if !node.StartsAt.IsZero() {
		query = query.Set("starts_at", node.StartsAt)
	}
	if !node.EndsAt.IsZero() {
		query = query.Set("ends_at", node.EndsAt)
	}

	if node.OwnerID > 0 && node.OwnerType != "" {
		query = query.Set("owner_type", node.OwnerType)
	}

	query = query.Set("comment_status", node.CommentStatus)
	// Left as it is unless a value is given. "Whoever the profile lets
	// reply" is given as the word "anyone", so it can be set back.
	if node.ReplyAccess != "" {
		query = query.Set("reply_access", node.ReplyAccess)
	}
	// The same: left as it is unless a value is given, and "anyone" is how
	// it is set back. What a note quotes is set once, when it is made, and
	// is not written here at all.
	if node.QuotePolicy != "" {
		query = query.Set("quote_policy", node.QuotePolicy)
	}
	query = query.Set("comment_count", node.CommentCount)
	query = query.Set("enabled", node.Enabled)
	query = query.Set("verified", node.Verified)
	query = query.Set("website_url", node.WebsiteURL)
	// person_type is deliberately not here: it is an authority level, not a
	// profile field. See applyEditableActorFields.
	query = query.Set("person_pronouns", node.PersonPronouns)
	// Not pinned: a post is pinned and unpinned through PinsRepo, which
	// keeps the limit and the date. An edit used to write the flag from
	// whatever it carried.

	if node.Meta != nil {
		query = query.Set("meta", node.Meta)
	}

	if node.ViewCount > 0 {
		query = query.Set("view_count", node.ViewCount)
	}

	if node.ParentID > 0 && node.ParentType != "" {
		query = query.Set("parent_id", node.ParentID)
		query = query.Set("parent_type", node.ParentType)
	}

	if node.GeoLatitude != 0 {
		query = query.Set("geo_latitude", node.GeoLatitude)
	}
	if node.GeoLongitude != 0 {
		query = query.Set("geo_longitude", node.GeoLongitude)
	}
	if node.GeoAddress != "" {
		query = query.Set("geo_address", node.GeoAddress)
	}
	if node.GeoCity != "" {
		query = query.Set("geo_city", node.GeoCity)
	}
	if node.GeoStateProvince != "" {
		query = query.Set("geo_state_province", node.GeoStateProvince)
	}
	if node.GeoCountry != "" {
		query = query.Set("geo_country", node.GeoCountry)
	}
	if node.GeoPostalcode != "" {
		query = query.Set("geo_postalcode", node.GeoPostalcode)
	}

	query = query.Set("modified_by", node.ModifiedBy)
	query = query.Set("updated_at", time.Now())

	if node.Filename != "" {
		query = query.Set("filename", node.Filename)
	}
	if node.AvatarFilename != "" {
		query = query.Set("avatar_filename", node.AvatarFilename)
	}
	if node.CoverFilename != "" {
		query = query.Set("cover_filename", node.CoverFilename)
	}
	// Written whichever way it is, like the setting below it. It was only
	// written when true, so once somebody let people ask to follow them
	// there was no way to stop.
	query = query.Set("allow_follow_request", node.AllowFollowRequest)
	// Written whichever way it is, so that it can be turned off again. An
	// edit of anything else carries what was read, and leaves it as it was.
	query = query.Set("show_reposts_on_profile", node.ShowRepostsOnProfile)
	if node.Excerpt != "" {
		query = query.Set("excerpt", node.Excerpt)
	}
	if node.Mimetype != "" {
		query = query.Set("mimetype", node.Mimetype)
	}
	if node.Language != "" {
		query = query.Set("language", node.Language)
	}
	if node.Access != "" {
		query = query.Set("access", node.Access)
	}
	if node.FeaturesRaw != nil {
		query = query.Set("features", node.FeaturesRaw)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	_, err = r.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	// log.Println("Node edited successfully:", node.ID)

	return r.Read(node.ID)
}

// BatchEdit updates multiple existing nodes based on the provided entities.
func (r *nodesRepo) BatchEdit(nodes []*entities.AnahitaNode) ([]*entities.AnahitaNode, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	var updatedNodes []*entities.AnahitaNode
	for _, node := range nodes {
		updatedNode, err := r.Edit(node)
		if err != nil {
			return nil, err
		}
		updatedNodes = append(updatedNodes, updatedNode)
	}

	return updatedNodes, nil
}
