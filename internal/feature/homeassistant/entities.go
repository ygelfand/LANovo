package homeassistant

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type Entity struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	State  string   `json:"state"`
	AreaID string   `json:"area_id"`
	Area   string   `json:"area"`
	Labels []string `json:"labels"`
}

func (e Entity) Domain() string {
	domain, _, _ := strings.Cut(e.ID, ".")
	return domain
}

func (e Entity) Available() bool { return e.State != "unavailable" }

type Filter struct {
	Domains []string
	Classes []string
	Labels  []string
}

const entitiesTemplate = `{%- set ns = namespace(out=[]) -%}
{%- for s in SOURCE -%}
{%- if (CLASSES | length == 0 or s.attributes.device_class in CLASSES) and (LABELS | length == 0 or labels(s.entity_id) | select('in', LABELS) | list | length > 0) -%}
{%- set ns.out = ns.out + [{'id': s.entity_id, 'name': s.name, 'state': s.state, 'area_id': area_id(s.entity_id) or '', 'area': area_name(s.entity_id) or '', 'labels': labels(s.entity_id)}] -%}
{%- endif -%}
{%- endfor -%}
{{ ns.out | tojson }}`

func (f Filter) template() string {
	source := "states"
	if len(f.Domains) > 0 {
		source = "states | selectattr('domain', 'in', " + list(f.Domains) + ")"
	}
	return strings.NewReplacer(
		"SOURCE", source,
		"CLASSES", list(f.Classes),
		"LABELS", list(f.Labels),
	).Replace(entitiesTemplate)
}

func list(values []string) string {
	if values == nil {
		values = []string{}
	}
	out, _ := json.Marshal(values)
	return string(out)
}

func (h *HomeAssistant) Entities(ctx context.Context, f Filter) ([]Entity, error) {
	raw, err := h.Template(ctx, f.template())
	if err != nil {
		return nil, err
	}
	return parseEntities(raw)
}

func unwrap(raw json.RawMessage) json.RawMessage {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return json.RawMessage(text)
	}
	return raw
}

func parseEntities(raw json.RawMessage) ([]Entity, error) {
	out := []Entity{}
	if err := json.Unmarshal(unwrap(raw), &out); err != nil {
		return nil, fmt.Errorf("homeassistant: the entity list: %w", err)
	}
	slices.SortFunc(out, func(a, b Entity) int {
		if (a.Area == "") != (b.Area == "") {
			if a.Area == "" {
				return 1
			}
			return -1
		}
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Area), strings.ToLower(b.Area)),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID),
		)
	})
	return out, nil
}
