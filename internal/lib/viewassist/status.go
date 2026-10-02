package viewassist

import "strings"

// Status items are the icons along the edge of a View Assist screen: a camera that is recording, a
// door that is open, a shortcut to another view. add_status_item and remove_status_item carry them
// as strings in one of three forms, which is what this reads.
//
//	view:target|icon        go to a view, or to a path outside the dashboard
//	entity:entity_id|icon   show an entity, and its state decides how the icon is drawn
//	service:service_name|icon   call a service
//
// Anything else comes back not ok. Their docs describe these three and nothing else, and the rest
// of this package follows the same rule: what is documented is written down, and what is not waits
// for a real instance rather than being invented.

// StatusKind is what a status item points at.
type StatusKind string

const (
	StatusView    StatusKind = "view"
	StatusEntity  StatusKind = "entity"
	StatusService StatusKind = "service"
)

// StatusKinds is every kind there is, in the order the documentation lists them.
var StatusKinds = []StatusKind{StatusView, StatusEntity, StatusService}

// StatusItem is one icon.
type StatusItem struct {
	Kind StatusKind

	// Target is the view path, entity id or service name, whichever the kind says.
	Target string

	// Icon is the mdi name to draw, which may be absent: the icon is the optional half of the
	// form, and an item without one is for the screen to choose for.
	Icon string
}

// ParseStatus reads one status item.
//
// The icon is split on the first bar rather than the last, because a service name cannot contain
// one and an icon name cannot either — so a second bar is part of neither and the string is not one
// of the three forms.
func ParseStatus(s string) (StatusItem, bool) {
	s = strings.TrimSpace(s)

	kind, rest, ok := strings.Cut(s, ":")
	if !ok {
		return StatusItem{}, false
	}

	at := StatusKind(strings.ToLower(strings.TrimSpace(kind)))
	switch at {
	case StatusView, StatusEntity, StatusService:
	default:
		return StatusItem{}, false
	}

	target, icon, _ := strings.Cut(rest, "|")
	target = strings.TrimSpace(target)
	icon = strings.TrimSpace(icon)

	if target == "" || strings.Contains(icon, "|") {
		return StatusItem{}, false
	}
	return StatusItem{Kind: at, Target: target, Icon: icon}, true
}

// String writes the item back in the form it was read from, so what is sent on is what arrived.
func (s StatusItem) String() string {
	at := string(s.Kind) + ":" + s.Target
	if s.Icon == "" {
		return at
	}
	return at + "|" + s.Icon
}

// View is the view a view item names, for the ones that name one.
//
// Not every view item does: their docs have these carrying external paths as well, which is the
// same reason Navigated reports whether a path is a view rather than assuming it is.
func (s StatusItem) View() (View, bool) {
	if s.Kind != StatusView {
		return "", false
	}
	return Navigated(s.Target)
}
