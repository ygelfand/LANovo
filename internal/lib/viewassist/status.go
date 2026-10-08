package viewassist

import "strings"

// View Assist status items: view:target|icon, entity:entity_id|icon, service:service_name|icon.

type StatusKind string

const (
	StatusView    StatusKind = "view"
	StatusEntity  StatusKind = "entity"
	StatusService StatusKind = "service"
)

var StatusKinds = []StatusKind{StatusView, StatusEntity, StatusService}

type StatusItem struct {
	Kind StatusKind

	Target string

	Icon string
}

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

func (s StatusItem) String() string {
	at := string(s.Kind) + ":" + s.Target
	if s.Icon == "" {
		return at
	}
	return at + "|" + s.Icon
}

func (s StatusItem) View() (View, bool) {
	if s.Kind != StatusView {
		return "", false
	}
	return Navigated(s.Target)
}
