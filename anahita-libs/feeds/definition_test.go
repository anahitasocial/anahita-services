package feeds

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseTidiesAndKeeps(t *testing.T) {
	def, err := Parse([]byte(`{
		"sources": {"following": true, "actors": [5, 5, 9]},
		"filters": {"media_types": ["photo", "note", "photo"], "hashtags_any": [" #Garden ", "seeds", "GARDEN"], "keywords_any": ["  heirloom   tomato "], "language": "FR"},
		"exclude": {"keywords": ["spam"]}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprint(def.Sources.Actors); got != "[5 9]" {
		t.Errorf("actors %s", got)
	}
	if got := fmt.Sprint(def.Filters.Hashtags); got != "[garden seeds]" {
		t.Errorf("hashtags %s", got)
	}
	if got := fmt.Sprint(def.Filters.Keywords); got != "[heirloom tomato]" {
		t.Errorf("keywords %s", got)
	}
	if got := fmt.Sprint(def.Filters.MediaTypes); got != "[photo note]" {
		t.Errorf("media types %s", got)
	}
	if def.Filters.Language != "fr" || def.Order != OrderNewest || def.IncludeReplies {
		t.Errorf("language %q, order %q, replies %v", def.Filters.Language, def.Order, def.IncludeReplies)
	}

	// What is stored reads back the same.
	again, err := Parse(def.JSON())
	if err != nil {
		t.Fatal(err)
	}
	if string(again.JSON()) != string(def.JSON()) {
		t.Errorf("stored form changed on a second reading:\n%s\n%s", def.JSON(), again.JSON())
	}
	if !strings.Contains(string(def.JSON()), `"locations_any":[]`) {
		t.Errorf("an empty list was left out: %s", def.JSON())
	}
}

func TestParseRefuses(t *testing.T) {
	many := func(n int, item func(i int) string) string {
		items := make([]string, n)
		for i := range items {
			items[i] = item(i)
		}
		return "[" + strings.Join(items, ",") + "]"
	}
	number := func(i int) string { return fmt.Sprint(i + 1) }
	word := func(i int) string { return fmt.Sprintf(`"word%d"`, i) }

	cases := map[string]struct {
		raw  string
		want Refusal
	}{
		"nothing":            {``, RefusedMalformed},
		"not json":           {`{`, RefusedMalformed},
		"two values":         {`{"sources":{"following":true}} {}`, RefusedMalformed},
		"a key not known":    {`{"sources":{"following":true},"sql":"1=1"}`, RefusedUnknownField},
		"a nested key":       {`{"sources":{"following":true,"everyone":true}}`, RefusedUnknownField},
		"the whole site":     {`{}`, RefusedTooBroad},
		"every photo":        {`{"filters":{"media_types":["photo"]}}`, RefusedTooBroad},
		"every group's post": {`{"filters":{"actor_types":["group"]}}`, RefusedTooBroad},
		"a kind of post":     {`{"sources":{"following":true},"filters":{"media_types":["podcast"]}}`, RefusedMediaType},
		"a kind of profile":  {`{"sources":{"following":true},"filters":{"actor_types":["website"]}}`, RefusedActorType},
		"an order":           {`{"sources":{"following":true},"order":"top"}`, RefusedOrder},
		"a language":         {`{"sources":{"following":true},"filters":{"language":"en'; --"}}`, RefusedLanguage},
		"a short keyword":    {`{"filters":{"keywords_any":["a"]}}`, RefusedKeywordLength},
		"a hashtag":          {`{"filters":{"hashtags_any":["two words"]}}`, RefusedHashtag},
		"an actor":           {`{"sources":{"actors":[0]}}`, RefusedActor},
		"a place":            {`{"filters":{"locations_any":[-1]}}`, RefusedLocation},
		"too many actors":    {`{"sources":{"actors":` + many(MaxActors+1, number) + `}}`, RefusedTooManyActors},
		"too many hashtags":  {`{"filters":{"hashtags_any":` + many(MaxHashtags+1, word) + `}}`, RefusedTooManyTags},
		"too many keywords":  {`{"filters":{"keywords_any":` + many(MaxKeywords+1, word) + `}}`, RefusedTooManyWords},
		"too many places":    {`{"filters":{"locations_any":` + many(MaxLocations+1, number) + `}}`, RefusedTooManyPlaces},
		"too many left out":  {`{"sources":{"following":true},"exclude":{"hashtags":` + many(MaxHashtags+1, word) + `}}`, RefusedTooManyTags},
	}

	for name, c := range cases {
		_, err := Parse([]byte(c.raw))
		got, ok := AsRefusal(err)
		if !ok || got != c.want {
			t.Errorf("%s: got %v, want %s", name, err, c.want)
		}
	}
}

// The whole site may be looked at through a hashtag, a keyword or a place.
func TestParseAllowsTheSiteThroughANarrowing(t *testing.T) {
	for _, raw := range []string{
		`{"filters":{"hashtags_any":["garden"]}}`,
		`{"filters":{"keywords_any":["compost"]}}`,
		`{"filters":{"locations_any":[42]}}`,
		`{"filters":{"actor_types":["group"],"hashtags_any":["garden"]}}`,
	} {
		if _, err := Parse([]byte(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

func TestEntityTypes(t *testing.T) {
	def := &Definition{Sources: Sources{Following: true}}
	if err := def.Check(); err != nil {
		t.Fatal(err)
	}
	if got := len(def.MediaEntityTypes()); got != 5 {
		t.Errorf("no kind chosen is every kind, got %d", got)
	}
	if got := len(def.ActorEntityTypes()); got != 0 {
		t.Errorf("no kind of profile chosen is any, got %d", got)
	}

	def.Filters.MediaTypes = []string{"photo"}
	def.Filters.ActorTypes = []string{"event", "group"}
	if got := fmt.Sprint(def.MediaEntityTypes()); got != "[node.medium.photo-service.photo.v1]" {
		t.Errorf("media %s", got)
	}
	if got := fmt.Sprint(def.ActorEntityTypes()); got != "[node.actor.actor-service.event.v1 node.actor.group-service.group.v1]" {
		t.Errorf("actors %s", got)
	}

	// People are people here and on other servers alike.
	def.Filters.ActorTypes = []string{"person"}
	if got := fmt.Sprint(def.ActorEntityTypes()); got != "[node.actor.person-service.person.v1 node.actor.activitypub-service.person.v1 node.actor.atproto-service.person.v1]" {
		t.Errorf("people %s", got)
	}
}

func TestCursorGoesThereAndBack(t *testing.T) {
	if got := (Cursor{}).String(); got != "" {
		t.Errorf("the start of a feed is %q", got)
	}

	start, err := ParseCursor("")
	if err != nil || !start.IsZero() {
		t.Errorf("empty: %v %v", start, err)
	}

	sent := Cursor{CreatedAt: time.Unix(1790000000, 0).UTC(), ID: 4217}
	back, err := ParseCursor(sent.String())
	if err != nil || !back.CreatedAt.Equal(sent.CreatedAt) || back.ID != sent.ID {
		t.Errorf("got %v %v, want %v", back, err, sent)
	}

	for _, bad := range []string{"!!", "bm90IGEgY3Vyc29y", "MTIz", "MC4w"} {
		if _, err := ParseCursor(bad); err == nil {
			t.Errorf("%q was read as a cursor", bad)
		}
	}
}

func TestStoredGoesThereAndBack(t *testing.T) {
	def, err := Parse([]byte(`{"filters":{"hashtags_any":["garden"]}}`))
	if err != nil {
		t.Fatal(err)
	}

	back, err := ReadStored(NewStored(def, "hashtag").JSON())
	if err != nil || back.Icon != "hashtag" || string(back.Definition.JSON()) != string(def.JSON()) {
		t.Fatalf("read back %+v (%v)", back, err)
	}

	// A picture that is not one of ours is the default.
	if got := NewStored(def, "skull").Icon; got != DefaultIcon {
		t.Errorf("an unknown picture was kept as %q", got)
	}
	unknown, err := ReadStored([]byte(`{"definition":{"sources":{"following":true}},"icon":"skull"}`))
	if err != nil || unknown.Icon != DefaultIcon {
		t.Errorf("an unknown picture was read as %+v (%v)", unknown, err)
	}

	// A definition alone, as the first feeds were kept.
	old, err := ReadStored(def.JSON())
	if err != nil || old.Icon != DefaultIcon || len(old.Definition.Filters.Hashtags) != 1 {
		t.Errorf("a definition alone was read as %+v (%v)", old, err)
	}

	// What is wrong with a definition is still what is said.
	for raw, want := range map[string]Refusal{
		`{"definition":{},"icon":"feed"}`:          RefusedTooBroad,
		`{"definition":{"sql":"1"},"icon":"feed"}`: RefusedUnknownField,
		`not json`: RefusedMalformed,
		`{"sources":{"following":true},"extra":true}`: RefusedUnknownField,
	} {
		_, err := ReadStored([]byte(raw))
		if got, ok := AsRefusal(err); !ok || got != want {
			t.Errorf("%s: got %v, want %s", raw, err, want)
		}
	}

	if !ValidIcon("feed") || ValidIcon("") || ValidIcon("skull") {
		t.Error("which pictures there are is wrong")
	}
}
