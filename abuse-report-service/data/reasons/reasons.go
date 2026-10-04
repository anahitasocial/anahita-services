// Package reasons is the list of reasons somebody can give when they report
// something.
//
// The list is a JSON file, because it is the installation's to decide: what a
// community forbids is its own business, and so is the language it explains
// it in. The service carries a default list, built in, which is used when
// REASONS_PATH is not set.
//
// An installation's own file is read again whenever it changes, so editing it
// takes effect without a restart. A file that cannot be used, because it is
// missing, is not JSON, or breaks a rule below, is never half-applied: the
// built-in list is used and the log says why.
package reasons

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed reasons.json
var builtIn []byte

// DefaultLanguage is the language every reason must have a label in, and the
// one used when the reader's language is not offered.
const DefaultLanguage = "en-GB"

// Reason is one thing somebody can report something for.
type Reason struct {
	// Key is what is stored on a report. Changing it orphans the label of
	// every report that used the old one, so treat it as permanent.
	Key string `json:"key"`

	// Label and Description, by language tag.
	Label       map[string]string `json:"label"`
	Description map[string]string `json:"description,omitempty"`

	// AppliesTo limits the reason to some kinds of node, by type prefix:
	// "node.actor." for people and groups, "node.medium." for posts. Empty
	// means everything.
	AppliesTo []string `json:"applies_to,omitempty"`

	// RequiresDetails makes the reporter explain. For "something else".
	RequiresDetails bool `json:"requires_details,omitempty"`

	// AtprotoReason is the AT Protocol reason this maps to, for the day
	// reports are forwarded to other servers. Not used yet.
	AtprotoReason string `json:"atproto_reason,omitempty"`
}

// Applies reports whether the reason can be given for a node of this type.
func (r Reason) Applies(nodeType string) bool {
	if len(r.AppliesTo) == 0 {
		return true
	}
	for _, prefix := range r.AppliesTo {
		if strings.HasPrefix(nodeType, prefix) {
			return true
		}
	}
	return false
}

// LabelIn is the label in the given language, or in the default one.
func (r Reason) LabelIn(language string) string {
	return inLanguage(r.Label, language)
}

// DescriptionIn is the description in the given language, or in the default.
func (r Reason) DescriptionIn(language string) string {
	return inLanguage(r.Description, language)
}

func inLanguage(texts map[string]string, language string) string {
	if text := texts[language]; text != "" {
		return text
	}
	// "fr" for a reader asking for "fr-CA", when only "fr-FR" is offered.
	if primary, _, found := strings.Cut(language, "-"); found || primary != "" {
		for tag, text := range texts {
			if strings.HasPrefix(tag, primary+"-") && text != "" {
				return text
			}
		}
	}
	return texts[DefaultLanguage]
}

// Catalogue is a validated list of reasons.
type Catalogue struct {
	Reasons []Reason `json:"reasons"`
}

// For returns the reasons that can be given for a node of this type, in the
// file's order.
func (c *Catalogue) For(nodeType string) []Reason {
	applicable := make([]Reason, 0, len(c.Reasons))
	for _, reason := range c.Reasons {
		if reason.Applies(nodeType) {
			applicable = append(applicable, reason)
		}
	}
	return applicable
}

// Find returns the reason with this key.
func (c *Catalogue) Find(key string) (Reason, bool) {
	for _, reason := range c.Reasons {
		if reason.Key == key {
			return reason, true
		}
	}
	return Reason{}, false
}

// Parse reads a reasons file and checks it. The rules:
//
//   - at least one reason;
//   - every key present, at most 64 characters, and used once;
//   - every reason labelled in the default language.
func Parse(data []byte) (*Catalogue, error) {
	catalogue := &Catalogue{}
	if err := json.Unmarshal(data, catalogue); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}

	if len(catalogue.Reasons) == 0 {
		return nil, errors.New("no reasons")
	}

	seen := map[string]bool{}
	for index, reason := range catalogue.Reasons {
		switch {
		case reason.Key == "":
			return nil, fmt.Errorf("reason %d has no key", index+1)
		case len(reason.Key) > 64:
			return nil, fmt.Errorf("key %q is longer than 64 characters", reason.Key)
		case seen[reason.Key]:
			return nil, fmt.Errorf("key %q is used twice", reason.Key)
		case reason.Label[DefaultLanguage] == "":
			return nil, fmt.Errorf("reason %q has no %s label", reason.Key, DefaultLanguage)
		}
		seen[reason.Key] = true
	}

	return catalogue, nil
}

// BuiltIn is the list the service carries.
func BuiltIn() *Catalogue {
	catalogue, err := Parse(builtIn)
	if err != nil {
		// The built-in list is part of the build. If it is broken there is
		// nothing sensible to run with.
		panic("reasons: the built-in list is invalid: " + err.Error())
	}
	return catalogue
}

// Source hands out the current list: the installation's file while it is
// usable, the built-in one otherwise.
type Source struct {
	path string

	mu       sync.Mutex
	modified time.Time
	size     int64
	current  *Catalogue
	fallback *Catalogue
}

// NewSource reads reasons from path, or uses the built-in list when path is
// empty.
func NewSource(path string) *Source {
	return &Source{path: path, fallback: BuiltIn()}
}

// Catalogue returns the list in force. It looks at the file on every call,
// and re-reads it only when it has changed.
func (s *Source) Catalogue() *Catalogue {
	if s.path == "" {
		return s.fallback
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(s.path)
	if err != nil {
		if s.current != nil || !s.modified.IsZero() {
			log.Printf("reasons: %s can no longer be read, using the built-in list: %v", s.path, err)
		} else {
			log.Printf("reasons: %s cannot be read, using the built-in list: %v", s.path, err)
		}
		s.current, s.modified, s.size = nil, time.Unix(1, 0), 0
		return s.fallback
	}

	if info.ModTime().Equal(s.modified) && info.Size() == s.size {
		if s.current != nil {
			return s.current
		}
		return s.fallback
	}

	s.modified, s.size = info.ModTime(), info.Size()

	data, err := os.ReadFile(s.path)
	if err == nil {
		s.current, err = Parse(data)
	}
	if err != nil {
		log.Printf("reasons: %s is not usable, using the built-in list: %v", s.path, err)
		s.current = nil
		return s.fallback
	}

	log.Printf("reasons: loaded %d reasons from %s", len(s.current.Reasons), s.path)
	return s.current
}
