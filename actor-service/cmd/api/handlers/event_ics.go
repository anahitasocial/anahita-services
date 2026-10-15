package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"anahita-libs/data/entities"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// An event as a calendar file, for "Add to calendar".
//
// One VEVENT in iCalendar (RFC 5545). Its times are written in UTC: they
// are stored as instants, an instant is the same everywhere, and every
// calendar shows it in its owner's own zone. That is right across a clock
// change without the file having to describe the zone's rules.

const icsTime = "20060102T150405Z"

// icsEscape makes text safe as an iCalendar value: a backslash, a comma
// and a semicolon each mean something there, and a line break ends the
// property.
func icsEscape(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, "\r\n", "\\n")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, "\r", "\\n")
	return text
}

// icsFold breaks a line at 75 octets, as the format asks, never in the
// middle of a character. A line carried on starts with a space.
func icsFold(line string) string {
	const most = 75

	if len(line) <= most {
		return line
	}

	var (
		out   strings.Builder
		width int
	)
	for len(line) > 0 {
		r, size := utf8.DecodeRuneInString(line)
		if width+size > most {
			out.WriteString("\r\n ")
			// The space that starts it counts towards the next line.
			width = 1
		}
		out.WriteRune(r)
		width += size
		line = line[size:]
	}

	return out.String()
}

// eventICS writes an event as a calendar file.
//
// link is the event's page, or "" to leave it out. onlineURL is where to
// join it and address where it is held, each given only when this viewer
// may be shown it.
func eventICS(event *entities.AnahitaNode, details *entities.EventDetails, link string, onlineURL string, address string, host string, now time.Time) string {
	if host == "" {
		host = "anahita"
	}

	description := event.Body
	// Held somewhere and online as well: the calendar's place is where it
	// is held, so the way in online goes with the description.
	if address != "" && onlineURL != "" {
		if description != "" {
			description += "\n\n"
		}
		description += onlineURL
	}
	if link != "" {
		if description != "" {
			description += "\n\n"
		}
		description += link
	}

	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Anahita//Events//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
		fmt.Sprintf("UID:event-%d@%s", event.ID, host),
		"DTSTAMP:" + now.UTC().Format(icsTime),
		"DTSTART:" + event.StartsAt.UTC().Format(icsTime),
		"DTEND:" + event.EndsAt.UTC().Format(icsTime),
		"SUMMARY:" + icsEscape(event.Name),
	}

	if description != "" {
		lines = append(lines, "DESCRIPTION:"+icsEscape(description))
	}
	if address != "" {
		lines = append(lines, "LOCATION:"+icsEscape(address))
	} else if onlineURL != "" {
		lines = append(lines, "LOCATION:"+icsEscape(onlineURL))
	}
	if link != "" {
		lines = append(lines, "URL:"+link)
	}
	// A calendar that already has it shows it struck through, where one
	// given nothing would go on showing an event that is not happening.
	if details != nil && details.CancelledAt != nil {
		lines = append(lines, "STATUS:CANCELLED")
	} else {
		lines = append(lines, "STATUS:CONFIRMED")
	}

	lines = append(lines, "END:VEVENT", "END:VCALENDAR")

	for i, line := range lines {
		lines[i] = icsFold(line)
	}

	return strings.Join(lines, "\r\n") + "\r\n"
}

// eventLink is the address of an event's page in the web app, or "" when
// the service has not been told where the web app is.
func eventLink(origin string, event *entities.AnahitaNode) string {
	if origin == "" || event == nil {
		return ""
	}

	if event.Alias == "" {
		return fmt.Sprintf("%s/events/%d", origin, event.ID)
	}

	// At its alias, as every profile is.
	return fmt.Sprintf("%s/@%s", origin, event.Alias)
}

// ICS sends an event as a calendar file, to anybody who can see the event.
//
// GET /events/:id/ics
//
// Where to join online is in it only for whoever is going, or looks after
// the event, as on the event's page.
func (h *eventHandler) ICS(c echo.Context) error {
	var (
		req    = &eventIDRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanRead(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	h.setEventDetails([]*entities.AnahitaNode{actor}, viewer)
	if actor.Event == nil {
		return c.NoContent(http.StatusNotFound)
	}

	onlineURL, address := "", ""
	if actor.EventShowsOnlineURL {
		onlineURL = actor.Event.OnlineURL
		address = responses.NewEventAddress(actor).OneLine()
	}

	origin := h.Config.GetWebOrigin()
	host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")

	body := eventICS(actor, actor.Event, eventLink(origin, actor), onlineURL, address, host, time.Now())

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="event-%d.ics"`, actor.ID))

	return c.Blob(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}
