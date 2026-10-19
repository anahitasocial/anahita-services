package main

import (
	"log"
	"time"

	"notification/data/repositories"
)

const (
	// How often read notifications past keeping are looked for.
	keepingEvery = 6 * time.Hour
	// How many go at a time.
	keepingBatch = 1000
)

// keepNotifications removes read notifications once they have been read for
// longer than the installation keeps them (NOTIFICATIONS_KEEP_DAYS; roadmap
// row 97): at start, then every six hours. Unread ones stay, however old.
// Zero days keeps them all. notification-service runs as one copy, so this
// runs once.
func keepNotifications(repo repositories.NotificationInterface, days int) {
	if days <= 0 {
		log.Println("notifications: read ones are kept for good (NOTIFICATIONS_KEEP_DAYS=0)")
		return
	}
	for {
		removeReadBefore(repo, time.Now().AddDate(0, 0, -days))
		time.Sleep(keepingEvery)
	}
}

// removeReadBefore removes those read before then, a batch at a time.
func removeReadBefore(repo repositories.NotificationInterface, before time.Time) int64 {
	var removed int64
	for {
		n, err := repo.DeleteReadBefore(before, keepingBatch)
		if err != nil {
			log.Println("notifications: removing read ones past keeping:", err)
			return removed
		}
		removed += n
		if n < keepingBatch {
			break
		}
	}
	if removed > 0 {
		log.Printf("notifications: removed %d read before %s", removed, before.Format(time.RFC3339))
	}
	return removed
}
