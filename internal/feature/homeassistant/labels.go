package homeassistant

import (
	"context"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
)

type Label = sharedha.Label

func (h *HomeAssistant) Labels(ctx context.Context) ([]Label, error) {
	raw, err := h.Template(ctx, sharedha.LabelsTemplate)
	if err != nil {
		return nil, err
	}
	return sharedha.ParseLabels(raw)
}
