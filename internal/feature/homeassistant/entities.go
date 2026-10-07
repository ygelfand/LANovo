package homeassistant

import (
	"context"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
)

type Entity = sharedha.Entity
type Filter = sharedha.Filter

func (h *HomeAssistant) Entities(ctx context.Context, f Filter) ([]Entity, error) {
	raw, err := h.Template(ctx, f.Template())
	if err != nil {
		return nil, err
	}
	return sharedha.ParseEntities(raw)
}
