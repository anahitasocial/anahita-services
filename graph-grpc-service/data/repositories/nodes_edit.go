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

	// Ensure StartDate and EndDate have default values if zero value
	if node.StartDate.IsZero() {
		node.StartDate = time.Now()
	}
	if node.EndDate.IsZero() {
		// Default end date: same as start date (adjust if different logic needed)
		node.EndDate = node.StartDate
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
	if !node.StartDate.IsZero() {
		query = query.Set("start_date", node.StartDate)
	}
	if !node.EndDate.IsZero() {
		query = query.Set("end_date", node.EndDate)
	}

	if node.OwnerID > 0 && node.OwnerType != "" {
		query = query.Set("owner_type", node.OwnerType)
	}

	query = query.Set("comment_status", node.CommentStatus)
	query = query.Set("comment_count", node.CommentCount)
	query = query.Set("ordering", node.Ordering)
	query = query.Set("enabled", node.Enabled)
	query = query.Set("verified", node.Verified)
	query = query.Set("website_url", node.WebsiteURL)
	query = query.Set("pinned", node.Pinned)
	query = query.Set("is_default", node.IsDefault)

	if node.Meta != nil {
		query = query.Set("meta", node.Meta)
	}

	if node.HitsCount > 0 {
		query = query.Set("hits", node.HitsCount)
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
	query = query.Set("modified_on", time.Now())

	if node.Filename != "" {
		query = query.Set("filename", node.Filename)
	}
	if node.Filesize > 0 {
		query = query.Set("filesize", node.Filesize)
	}
	if node.CoverFilename != "" {
		query = query.Set("cover_filename", node.CoverFilename)
	}
	if node.CoverFilesize > 0 {
		query = query.Set("cover_filesize", node.CoverFilesize)
	}
	if node.CoverMimetype != "" {
		query = query.Set("cover_mimetype", node.CoverMimetype)
	}
	if node.Status != "" {
		query = query.Set("status", node.Status)
		query = query.Set("status_update_time", time.Now())
	}
	if node.AllowFollowRequest {
		query = query.Set("allow_follow_request", node.AllowFollowRequest)
	}
	if node.Excerpt != "" {
		query = query.Set("excerpt", node.Excerpt)
	}
	if node.Mimetype != "" {
		query = query.Set("mimetype", node.Mimetype)
	}
	if node.Timezone != 0 {
		query = query.Set("timezone", node.Timezone)
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
