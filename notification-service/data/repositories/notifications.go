package repositories

import (
	an_entities "anahita-libs/data/entities"
	"fmt"
	"log"
	"time"

	an_const "anahita-libs/constants"
	"context"
	"database/sql"
	"notification/constants"
	"notification/data/entities"
	"notification/requests"

	sq "github.com/Masterminds/squirrel"
)

type NotificationInterface interface {
	// Count returns the number of Notifications
	Count(req *requests.NotificationBrowseRequest, viewer *an_entities.Viewer) (uint64, error)

	// UnreadCount returns the number of unread Notifications
	UnreadCount(viewer *an_entities.Viewer) (uint64, error)

	// Browse returns a list of Notifications
	Browse(req *requests.NotificationBrowseRequest, viewer *an_entities.Viewer) ([]*entities.Notification, error)

	// ReadByID returns a story by its ID
	ReadByID(id int64) (*entities.Notification, error)

	// Edit
	Edit(notification *entities.Notification) error

	// Add a new notification
	Add(notification *entities.Notification) (int64, error)

	// DeleteOfReply removes the news of a reply that is gone.
	DeleteOfReply(replyID int64) error

	// Delete a notification
	Delete(notification *entities.Notification) error
}

// NewNotifications creates a new Notification repository
func NewNotification(db *sql.DB) NotificationInterface {
	return &Notification{
		entityType: constants.NodeBaseNotificationType,
		db:         db,
		dbTimeout:  an_const.DBTimeout,
	}
}

// Notifications is the Notification repository
type Notification struct {
	db         *sql.DB
	dbTimeout  time.Duration
	entityType string
}

// getBaseSelectQuery returns the base select query
func (r *Notification) getBaseSelectQuery() sq.SelectBuilder {
	var fields = []string{
		"notification.id",
		"notification.type",
		"notification.service",
		"notification.recipient_id",
		"notification.created_at",
		"notification.updated_at",
		"notification.is_read",
		// Who, what and where it is about: NULL when there is none, read as 0.
		"COALESCE(notification.subject_id, 0) AS subject_id",
		"COALESCE(notification.object_id, 0) AS object_id",
		"COALESCE(notification.target_id, 0) AS target_id",
	}

	return sq.Select(fields...).From(
		"notifications AS notification",
	)
}

// Count returns the number of Notifications
func (repo *Notification) Count(req *requests.NotificationBrowseRequest, viewer *an_entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := sq.Select("COUNT(*)").From(
		"notifications AS notification",
	)
	query = query.Where(sq.Eq{"notification.recipient_id": viewer.ID})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var count uint64
	err = repo.db.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// UnreadCount returns the number of unread Notifications
func (repo *Notification) UnreadCount(viewer *an_entities.Viewer) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	var unreadCount uint64
	err := repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notifications WHERE recipient_id = ? AND is_read = 0", viewer.ID).Scan(&unreadCount)
	if err != nil {
		return 0, err
	}

	return unreadCount, nil
}

// Browse returns a list of Notifications
func (repo *Notification) Browse(req *requests.NotificationBrowseRequest, viewer *an_entities.Viewer) ([]*entities.Notification, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery()
	query = query.Where(fmt.Sprintf("notification.recipient_id = %d", viewer.ID))

	// query = repo.SetPrivacyQuery(query, viewer)
	query = query.OrderBy("notification.created_at DESC")
	query = query.Limit(req.Limit).Offset(req.Offset)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*entities.Notification
	for rows.Next() {
		item := entities.NewNotification()
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Service,
			&item.RecipientID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.IsRead,
			&item.Subject.ID,
			&item.Object.ID,
			&item.Target.ID,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}

// ReadByID returns a Notification by id
func (repo *Notification) ReadByID(id int64) (*entities.Notification, error) {
	query := repo.getBaseSelectQuery().Where(sq.Eq{"notification.id": id})
	return repo.getItem(&query)
}

// Edit edits a Notification
func (repo *Notification) Edit(item *entities.Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := sq.Update("notifications").SetMap(sq.Eq{
		"is_read":    item.IsRead,
		"updated_at": time.Now(),
	}).Where(sq.Eq{"id": item.ID})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, sql, args...)
	return err
}

// addAgain is what telling somebody the same thing again does: one row for
// one thing (the unique key one_of_each), moved to the top of their list,
// unread. LAST_INSERT_ID(id) answers with that row's id.
const addAgain = `
ON DUPLICATE KEY UPDATE
    id = LAST_INSERT_ID(id),
    created_at = NOW(),
    updated_at = NOW(),
    is_read = 0`

// Add tells somebody something, or tells them again.
func (repo *Notification) Add(item *entities.Notification) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	query := sq.Insert("notifications").Columns(
		"type",
		"service",
		"recipient_id",
		"subject_id",
		"subject_type",
		"object_id",
		"object_type",
		"target_id",
		"target_type",
		"comment_id",
		"created_at",
		"updated_at",
	).Values(
		item.Type,
		item.Service,
		item.RecipientID,
		nullableID(item.Subject.ID),
		item.Subject.Type,
		nullableID(item.Object.ID),
		item.Object.Type,
		nullableID(item.Target.ID),
		item.Target.Type,
		nullableID(int64(item.CommentID)),
		time.Now(),
		time.Now(),
	).Suffix(addAgain)

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	log.Println("Executing create query:", sql, "with args:", args)

	result, err := repo.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

const deleteNotificationsOfReply = `
DELETE FROM notifications
WHERE comment_id = ?`

// DeleteOfReply removes the news of a reply, for everybody who was told:
// the reply is gone, and a notification that leads to nothing is noise.
// The reply's id is in comment_id, whoever it was news to.
func (repo *Notification) DeleteOfReply(replyID int64) error {
	if replyID <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteNotificationsOfReply, replyID)
	return err
}

// Delete a notification
func (repo *Notification) Delete(item *entities.Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	if item.ID > 0 {
		query := sq.Delete("notifications").Where(sq.Eq{"id": item.ID})

		sql, args, err := query.ToSql()
		if err != nil {
			return err
		}

		_, err = repo.db.ExecContext(ctx, sql, args...)
		if err != nil {
			return err
		}

		return nil
	}

	if item.Subject.ID > 0 && item.Target.ID > 0 {
		query := sq.Delete("notifications").Where(sq.Eq{
			"service":    item.Service,
			"subject_id": item.Subject.ID,
			"target_id":  item.Target.ID,
		})

		if item.RecipientID > 0 {
			query = query.Where(sq.Eq{"recipient_id": item.RecipientID})
		}

		// Only this kind, when one is named. Without it, everything between
		// the two goes: a request to follow being answered would take the
		// news of the follow with it.
		if item.Type != "" {
			query = query.Where(sq.Eq{"type": item.Type})
		}

		if item.CommentID > 0 {
			if item.Object.ID == 0 {
				return fmt.Errorf("Object.ID must exist if CommentID is provided")
			}
			query = query.Where(sq.Eq{
				"object_id":  item.Object.ID,
				"comment_id": item.CommentID,
			})
		} else if item.Object.ID > 0 {
			query = query.Where(sq.Eq{"object_id": item.Object.ID})
		}

		sql, args, err := query.ToSql()
		if err != nil {
			return err
		}

		log.Println("Executing delete query:", sql, "with args:", args)

		_, err = repo.db.ExecContext(ctx, sql, args...)
		if err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("invalid parameters for deletion")
}

// getItem returns a Notification by query
func (repo *Notification) getItem(q *sq.SelectBuilder) (*entities.Notification, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	item := entities.NewNotification()
	row := repo.db.QueryRowContext(ctx, sql, args...)
	err = row.Scan(
		&item.ID,
		&item.Type,
		&item.Service,
		&item.RecipientID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.IsRead,
		&item.Subject.ID,
		&item.Object.ID,
		&item.Target.ID,
	)

	if err != nil {
		return nil, err
	}

	return item, nil
}

// nullableID is the id, or NULL for none.
func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
