// Package say is the text the device shows, kept apart from the code that shows it.
//
// Every string on the panel used to be an English literal at its use site, which makes a
// translation impossible and makes the wording hard to review: there was no one place to read all
// of it and see that half the hints were narration.
//
// Identifiers are dotted and stable rather than the English text itself. Text as its own identifier
// is tempting and wrong here: the wording gets revised — several hints were rewritten the day
// before this was written — and every revision would orphan every translation of it.
package say

import (
	"embed"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Embedded rather than read from disk, the way the logo and the wake word models already are. A
// translation that can go missing from an install is one that will.
//
//go:embed messages/*.yaml
var files embed.FS

// English is what the device falls back to, and the only complete translation. A string a
// translation has not reached shows in English rather than as an identifier: a reader who meets one
// word in the wrong language has still been told something.
var English = language.English

var (
	mu     sync.RWMutex
	bundle *i18n.Bundle
	local  *i18n.Localizer
	chosen string

	// names is what each language calls itself, taken from its own messages.
	names = map[string]string{}

	// carries is which identifiers each language actually defines.
	//
	// Kept here because a localizer cannot answer it: go-i18n falls back to the default language
	// inside Localize, so asking one whether it has a string always says yes. That is the right
	// behaviour for drawing and the wrong one for reporting what a translation is missing.
	carries = map[string]map[string]bool{}

	// missed is the identifiers already complained about, so a string nobody translated is one line
	// in the log rather than one per draw.
	missed sync.Map
)

func init() {
	bundle = i18n.NewBundle(English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	entries, err := files.ReadDir("messages")
	if err != nil {
		slog.Error("no messages are embedded, so the interface will show identifiers", "err", err)
		return
	}

	for _, e := range entries {
		name := path.Join("messages", e.Name())

		raw, err := files.ReadFile(name)
		if err != nil {
			slog.Error("a message file would not read", "file", e.Name(), "err", err)
			continue
		}
		if err := add(strings.TrimSuffix(e.Name(), ".yaml"), name, raw); err != nil {
			slog.Error("a message file would not load", "file", e.Name(), "err", err)
		}
	}

	local = i18n.NewLocalizer(bundle, English.String())
	chosen = English.String()
}

// Use picks a language and reports what it settled on, which is English for one there are no
// messages for.
func Use(want string) string {
	mu.Lock()
	defer mu.Unlock()

	if want == "" {
		want = English.String()
	}

	// English last in every case, which is what makes a partial translation fall back per string
	// rather than all or nothing.
	local = i18n.NewLocalizer(bundle, want, English.String())
	chosen = settled(want)

	return chosen
}

// Chosen is the language in use.
func Chosen() string {
	mu.RLock()
	defer mu.RUnlock()
	return chosen
}

// add parses a message file and records which identifiers it defines.
func add(tag, name string, raw []byte) error {
	if _, err := bundle.ParseMessageFileBytes(raw, name); err != nil {
		return err
	}

	var flat map[string]any
	if err := yaml.Unmarshal(raw, &flat); err != nil {
		return fmt.Errorf("say: reading %s: %w", name, err)
	}

	has := map[string]bool{}
	for id := range flat {
		has[id] = true
	}
	carries[tag] = has

	if own, ok := flat["language.name"].(string); ok && own != "" {
		names[tag] = own
	}

	return nil
}

// Name is what a language calls itself, for the row that picks it. The tag itself for one that does
// not say, which is a translation to fix rather than a case to dress up.
func Name(tag string) string {
	mu.RLock()
	defer mu.RUnlock()

	if own := names[tag]; own != "" {
		return own
	}
	return tag
}

// settled is want if there are messages for it, otherwise English. What was actually loaded decides,
// so this answers with the truth rather than with what was asked for.
func settled(want string) string {
	if carries[want] != nil {
		return want
	}

	// A region asked for, messages held for the plain language: en-GB takes en.
	if base, _, ok := strings.Cut(want, "-"); ok && carries[base] != nil {
		return base
	}
	return English.String()
}

// Languages is every language there are messages for, English first and the rest in order.
func Languages() []string {
	mu.RLock()
	defer mu.RUnlock()

	var out []string
	for tag := range carries {
		if tag == English.String() {
			continue
		}
		out = append(out, tag)
	}
	sort.Strings(out)

	return append([]string{English.String()}, out...)
}

// T is the text for an identifier.
func T(id string) string { return look(id, &i18n.LocalizeConfig{MessageID: id}) }

// F is T with the values a message names filled in.
func F(id string, with map[string]any) string {
	return look(id, &i18n.LocalizeConfig{MessageID: id, TemplateData: with})
}

// N is T for a message that reads differently for one and for several. The count is also available
// to the message as .Count.
func N(id string, count int) string {
	return look(id, &i18n.LocalizeConfig{
		MessageID:    id,
		PluralCount:  count,
		TemplateData: map[string]any{"Count": count},
	})
}

// look resolves one message.
//
// An error alongside a string is the ordinary case rather than a failure: go-i18n reports that the
// chosen language did not have the identifier and hands back the default language's text in the
// same breath. That is the fallback working, so what comes back decides, not the error. Only an
// empty result means nothing anywhere defines it.
//
// An identifier nothing has is returned as itself. That is a bug rather than a state to handle, and
// a dotted identifier on the panel says which one loudly enough to find, where an empty row does
// not.
func look(id string, what *i18n.LocalizeConfig) string {
	mu.RLock()
	l := local
	mu.RUnlock()

	if l == nil {
		return id
	}

	out, err := l.Localize(what)
	if out != "" {
		return out
	}
	if err != nil {
		if _, said := missed.LoadOrStore(id, true); !said {
			slog.Warn("no text for an identifier", "id", id, "err", err)
		}
	}
	return id
}

// Ids is every identifier English defines, for a test that checks nothing asks for one that is not
// there.
func Ids() ([]string, error) {
	raw, err := files.ReadFile("messages/en.yaml")
	if err != nil {
		return nil, err
	}

	var flat map[string]any
	if err := yaml.Unmarshal(raw, &flat); err != nil {
		return nil, fmt.Errorf("say: reading en.yaml: %w", err)
	}

	out := make([]string, 0, len(flat))
	for id := range flat {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// Untranslated is every identifier English has that a language does not, for a test that reports
// how far a translation has got rather than failing over it.
func Untranslated(tag string) ([]string, error) {
	ids, err := Ids()
	if err != nil {
		return nil, err
	}

	mu.RLock()
	has := carries[tag]
	mu.RUnlock()

	var out []string
	for _, id := range ids {
		if !has[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// Load adds messages from bytes, for a test that wants a language without shipping one.
func Load(tag string, yamlBody string) error {
	mu.Lock()
	defer mu.Unlock()

	return add(tag, "messages/"+tag+".yaml", []byte(yamlBody))
}

// Missing reports whether an identifier reads as itself, which is what T does for one nothing
// defines.
//
// Quiet, unlike T: an optional message is asked for to find out whether it exists, and warning that
// it does not is the answer rather than a fault.
func Missing(id string) bool {
	if !strings.Contains(id, ".") {
		return false
	}

	mu.RLock()
	l := local
	mu.RUnlock()

	if l == nil {
		return true
	}

	out, _ := l.Localize(&i18n.LocalizeConfig{MessageID: id})
	return out == "" || out == id
}
