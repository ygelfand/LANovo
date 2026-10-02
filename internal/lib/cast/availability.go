package cast

import (
	"encoding/json"
	"fmt"
)

// TypeGetAppAvailability asks which applications this device can run. A sender lists the device
// only if its own application comes back available.
const TypeGetAppAvailability = "GET_APP_AVAILABILITY"

const (
	AppAvailable   = "APP_AVAILABLE"
	AppUnavailable = "APP_UNAVAILABLE"
)

type availabilityRequest struct {
	RequestID int      `json:"requestId"`
	AppID     []string `json:"appId"`
}

type availabilityResponse struct {
	ResponseType string            `json:"responseType"`
	RequestID    int               `json:"requestId"`
	Availability map[string]string `json:"availability"`
}

func (r *Receiver) availability(m Message) ([]Message, error) {
	var req availabilityRequest
	if err := json.Unmarshal([]byte(m.Payload), &req); err != nil {
		return nil, fmt.Errorf("cast: an availability request: %w", err)
	}

	out := availabilityResponse{
		ResponseType: TypeGetAppAvailability,
		RequestID:    req.RequestID,
		Availability: map[string]string{},
	}
	for _, app := range req.AppID {
		out.Availability[app] = AppAvailable
		if r.Available != nil && !r.Available(app) {
			out.Availability[app] = AppUnavailable
		}
	}

	body, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return []Message{reply(m, NSReceiver, string(body))}, nil
}
