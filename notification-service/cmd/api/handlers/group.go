package handlers

import (
	"fmt"

	"notification/data/entities"
)

// How many who did the same are named on a grouped notification: the newest
// and as many as two others. The rest are a number.
const groupNamed = 3

// groupKey is what makes notifications one when the list is read (roadmap
// row 97, part 3): likes of one post, reposts of one post, follows of one
// profile. The other kinds are each their own.
func groupKey(item *entities.Notification) string {
	switch item.Type {
	case NotificationTypeVoteup:
		return fmt.Sprintf("voteup:%d", item.Object.ID)
	case "repost_add":
		return fmt.Sprintf("repost_add:%d", item.Object.ID)
	case NotificationTypeActorFollow:
		return fmt.Sprintf("actor_follow:%d", item.Target.ID)
	}
	return ""
}

// groupNotifications makes one of each group on a page, at the place of its
// newest: "Worf and 2 others liked your post". Each keeps every row's id,
// so that reading or deleting it reads or deletes them all, and the next
// page begins after them. Unread while any of them is. Each row stays as it
// was stored; this is only how they are shown.
func groupNotifications(items []*entities.Notification) []*entities.Notification {
	grouped := make([]*entities.Notification, 0, len(items))
	first := map[string]*entities.Notification{}

	for _, item := range items {
		item.IDs = []int64{item.ID}
		key := groupKey(item)
		lead, seen := first[key]
		if key == "" || !seen {
			if key != "" {
				first[key] = item
			}
			grouped = append(grouped, item)
			continue
		}

		lead.IDs = append(lead.IDs, item.ID)
		lead.IsRead = lead.IsRead && item.IsRead
		if item.Subject.ID == lead.Subject.ID || namesAlready(lead, item.Subject.ID) {
			continue
		}
		if 1+len(lead.Others) < groupNamed {
			lead.Others = append(lead.Others, item.Subject)
		} else {
			lead.OthersCount++
		}
	}
	return grouped
}

func namesAlready(lead *entities.Notification, id int64) bool {
	for _, other := range lead.Others {
		if other.ID == id {
			return true
		}
	}
	return false
}
