package handlers

import (
	"strings"
	"testing"
	"time"

	"anahita-libs/data/entities"
)

func icsEvent() *entities.AnahitaNode {
	event := entities.NewAnahitaNode()
	event.ID = 42
	event.Name = "Picnic; bring food, drink"
	event.Alias = "picnic"
	event.Body = "By the lake.\nRain or shine."
	// 19:00 in Toronto on 5 November 2026, which is 00:00 UTC on the 6th.
	event.StartsAt = time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	event.EndsAt = event.StartsAt.Add(2 * time.Hour)
	return event
}

func TestEventICS(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	event := icsEvent()
	link := eventLink("https://example.org", event)

	got := eventICS(event, &entities.EventDetails{}, link, "", "", "example.org", now)

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n",
		"UID:event-42@example.org\r\n",
		"DTSTAMP:20261010T120000Z\r\n",
		// Instants in UTC, which every calendar shows in its owner's zone.
		"DTSTART:20261106T000000Z\r\n",
		"DTEND:20261106T020000Z\r\n",
		// What means something to the format is escaped.
		"SUMMARY:Picnic\\; bring food\\, drink\r\n",
		"URL:https://example.org/events/42-picnic\r\n",
		"STATUS:CONFIRMED\r\n",
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	// The description carries the line break as the format writes one, and
	// the way back to the event. Unfolded first, as a calendar reads it.
	unfolded := strings.ReplaceAll(got, "\r\n ", "")
	if !strings.Contains(unfolded, "DESCRIPTION:By the lake.\\nRain or shine.\\n\\nhttps://example.org/events/42-picnic\r\n") {
		t.Errorf("the description is not as expected:\n%s", unfolded)
	}

	// Where to join online is in it only when it was given.
	if strings.Contains(got, "LOCATION:") {
		t.Error("a location with nothing given")
	}
	with := eventICS(event, &entities.EventDetails{}, link, "https://meet.example/x", "", "example.org", now)
	if !strings.Contains(with, "LOCATION:https://meet.example/x\r\n") {
		t.Errorf("the online address is missing:\n%s", with)
	}

	// Held somewhere: that is the calendar's place, commas and all, and
	// the way in online moves to the description.
	held := eventICS(event, &entities.EventDetails{}, link, "https://meet.example/x", "123 Main St, Vancouver", "example.org", now)
	if !strings.Contains(held, "LOCATION:123 Main St\\, Vancouver\r\n") {
		t.Errorf("the address is not the place:\n%s", held)
	}
	if !strings.Contains(strings.ReplaceAll(held, "\r\n ", ""), "https://meet.example/x") {
		t.Errorf("the online address is lost when there is a place:\n%s", held)
	}

	// Called off.
	at := now
	off := eventICS(event, &entities.EventDetails{CancelledAt: &at}, link, "", "", "example.org", now)
	if !strings.Contains(off, "STATUS:CANCELLED\r\n") || strings.Contains(off, "STATUS:CONFIRMED") {
		t.Errorf("a called-off event does not say so:\n%s", off)
	}

	// No lines over 75 octets, and no bare line feeds.
	for _, line := range strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("a line of %d octets: %q", len(line), line)
		}
		if strings.Contains(line, "\n") {
			t.Errorf("a bare line feed in %q", line)
		}
	}
}

func TestICSFoldKeepsCharactersWhole(t *testing.T) {
	// Each of these is three octets, so 75 is not a multiple of a character.
	line := "SUMMARY:" + strings.Repeat("日", 60)
	folded := icsFold(line)

	for _, part := range strings.Split(folded, "\r\n") {
		if len(part) > 75 {
			t.Errorf("a part of %d octets", len(part))
		}
	}
	if got := strings.ReplaceAll(folded, "\r\n ", ""); got != line {
		t.Error("unfolding does not give the line back")
	}
}

func TestEventLink(t *testing.T) {
	event := icsEvent()

	if got := eventLink("https://example.org", event); got != "https://example.org/events/42-picnic" {
		t.Errorf("%q", got)
	}
	event.Alias = ""
	if got := eventLink("https://example.org", event); got != "https://example.org/events/42" {
		t.Errorf("with no alias: %q", got)
	}
	// Without the web app's address there is no link, and no broken one.
	if got := eventLink("", event); got != "" {
		t.Errorf("with no origin: %q", got)
	}
}
