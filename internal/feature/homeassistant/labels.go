package homeassistant

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const labelsTemplate = `{%- set ns = namespace(out=[]) -%}
{%- for l in labels() -%}
{%- set ns.out = ns.out + [{'id': l, 'name': label_name(l) or l}] -%}
{%- endfor -%}
{{ ns.out | tojson }}`

func (h *HomeAssistant) Labels(ctx context.Context) ([]Label, error) {
	raw, err := h.Template(ctx, labelsTemplate)
	if err != nil {
		return nil, err
	}
	return parseLabels(raw)
}

func parseLabels(raw json.RawMessage) ([]Label, error) {
	out := []Label{}
	if err := json.Unmarshal(unwrap(raw), &out); err != nil {
		return nil, fmt.Errorf("homeassistant: the label list: %w", err)
	}
	slices.SortFunc(out, func(a, b Label) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}
