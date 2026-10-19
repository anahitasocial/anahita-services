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

	// UnreadCount returns the number of unread Notifications, and when the
	// newest of all was told.
	UnreadCount(viewer *an_entities.Viewer) (uint64, *time.Time, error)

	// Browse returns a list of Notifications
	Browse(req *requests.NotificationBrowseRequest, viewer *an_entities.Viewer) ([]*entities.Notification, error)

	// ReadByID returns a story by its ID
	ReadByID(id int64) (*entities.Notification, error)

	// Edit
	Edit(notification *entities.Notification) error

	// EditAll marks all of the viewer's notifications read, and answers how
	// many were not.
	EditAll(viewer *an_entities.Viewer) (int64, error)

	// Add a new notification
	Add(notification *entities.Notification) (int64, error)

	// DeleteAbout removes every notification about a node that is gone.
	DeleteAbout(nodeID int64) error

	// DeleteReadBefore removes, a batch at a time, those read before then.
	DeleteReadBefore(before time.Time, batch int) (int64, error)

	// DeleteBetween removes what two profiles were told of each other.
	DeleteBetween(a, b int64) error

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
		// Read when read_at is set: NULL while it is unread.
		"notification.read_at IS NOT NULL AS is_read",
		// Who, what and where it is about: NULL when there is none, read as 0.
		"COALESCE(notification.subject_id, 0) AS subject_id",
		"COALESCE(notification.object_id, 0) AS object_id",
		"COALESCE(notification.target_id, 0) AS target_id",
		"COALESCE(notification.comment_id, 0) AS comment_id",
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

const unreadCountAndNewest = `
SELECT
    COALESCE(SUM(read_at IS NULL), 0),
    MAX(created_at)
FROM notifications
WHERE recipient_id = ?`

// UnreadCount returns how many of the viewer's notifications are unread, and
// when the newest of them all, read or not, was told: what the bell asks, and
// how an open list knows there is something new to fetch although the count
// is the same. Telling somebody something again moves its row's created_at,
// not its id, so the time is what says so. Nil when they have none.
func (repo *Notification) UnreadCount(viewer *an_entities.Viewer) (uint64, *time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	var unread uint64
	var newest sql.NullTime
	err := repo.db.QueryRowContext(ctx, unreadCountAndNewest, viewer.ID).Scan(&unread, &newest)
	if err != nil {
		return 0, nil, err
	}
	if !newest.Valid {
		return unread, nil, nil
	}
	return unread, &newest.Time, nil
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
			&item.CommentID,
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

	// Read now, or unread again. Read once, it keeps the time it was first.
	values := sq.Eq{"updated_at": time.Now()}
	if item.IsRead {
		values["read_at"] = sq.Expr("COALESCE(read_at, NOW())")
	} else {
		values["read_at"] = nil
	}
	query := sq.Update("notifications").SetMap(values).Where(sq.Eq{"id": item.ID})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, sql, args...)
	return err
}

const markAllRead = `
UPDATE notifications
SET read_at = NOW(),
    updated_at = NOW()
WHERE recipient_id = ?
    AND read_at IS NULL`

// EditAll marks every one of the viewer's notifications read (Mark all
// read), and answers how many were unread.
func (repo *Notification) EditAll(viewer *an_entities.Viewer) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, markAllRead, viewer.ID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// addAgain is what telling somebody the same thing again does: one row for
// one thing (the unique key one_of_each), moved to the top of their list,
// unread. LAST_INSERT_ID(id) answers with that row's id.
const addAgain = `
ON DUPLICATE KEY UPDATE
    id = LAST_INSERT_ID(id),
    created_at = NOW(),
    updated_at = NOW(),
    read_at = NULL`

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

const deleteNotificationsAbout = `
DELETE FROM notifications
WHERE object_id = ?
    OR comment_id = ?
    OR target_id = ?`

// DeleteAbout removes every notification about a node that is gone, for
// everybody who was told, whoever did what it tells of (roadmap row 96, part
// 2): a post deleted takes its likes, reposts, quotes, mentions and the
// replies under it; a reply or a repost takes the news of it, kept in
// comment_id. A notification that leads to nothing is noise. A node's id is
// the same in every column, so one id finds them all.
func (repo *Notification) DeleteAbout(nodeID int64) error {
	if nodeID <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteNotificationsAbout, nodeID, nodeID, nodeID)
	return err
}

const deleteReadBefore = `
DELETE FROM notifications
WHERE read_at < ?
LIMIT ?`

// DeleteReadBefore removes up to a batch of notifications read before the
// time given (NOTIFICATIONS_KEEP_DAYS): a batch at a time, so the table is
// never held long. Unread ones stay, however old.
func (repo *Notification) DeleteReadBefore(before time.Time, batch int) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	result, err := repo.db.ExecContext(ctx, deleteReadBefore, before, batch)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

const deleteNotificationsBetween = `
DELETE FROM notifications
WHERE (recipient_id = ? AND subject_id = ?)
    OR (recipient_id = ? AND subject_id = ?)
    OR (subject_id = ? AND target_id = ?)
    OR (subject_id = ? AND target_id = ?)`

// DeleteBetween removes what two profiles were told of each other, when one
// blocks the other (roadmap row 96, part 1): what each was told the other
// did, and, for a group or an event, what its administrators were told the
// other did to it.
func (repo *Notification) DeleteBetween(a, b int64) error {
	if a <= 0 || b <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), repo.dbTimeout)
	defer cancel()

	_, err := repo.db.ExecContext(ctx, deleteNotificationsBetween, a, b, b, a, b, a, a, b)
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
		&item.CommentID,
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
