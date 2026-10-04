package primevideo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	AuthHost   = "https://api.amazon.com"
	DeviceType = "A2Y2Z7THWOTN8I"
	appName    = "Prime Video"
	origin     = "https://cloudfront.xp-assets.aiv-cdn.net"
)

func Register(ctx context.Context, hc *http.Client, host, code, device string) (string, error) {
	body := map[string]any{
		"registration_data": map[string]string{
			"device_serial":    device,
			"os_version":       "Android",
			"app_name":         appName,
			"app_version":      "1.0",
			"device_model":     "Generic GCast",
			"device_name":      "GCast:" + DeviceType + "." + device,
			"device_type":      DeviceType,
			"domain":           "Device",
			"software_version": "1.0",
		},
		"auth_data":            map[string]string{"code": code},
		"requested_token_type": []string{"bearer"},
		"scopes":               []string{"aiv:full"},
	}
	var out struct {
		Response struct {
			Success struct {
				Tokens struct {
					Bearer struct {
						Refresh string `json:"refresh_token"`
					} `json:"bearer"`
				} `json:"tokens"`
			} `json:"success"`
		} `json:"response"`
	}
	if err := post(ctx, hc, host+"/auth/register", body, &out); err != nil {
		return "", fmt.Errorf("prime video: register: %w", err)
	}
	if out.Response.Success.Tokens.Bearer.Refresh == "" {
		return "", errors.New("prime video: register: no refresh token")
	}
	return out.Response.Success.Tokens.Bearer.Refresh, nil
}

func ActorToken(ctx context.Context, hc *http.Client, host, actor, refresh string) (string, error) {
	body := map[string]any{
		"actor_id":             actor,
		"app_name":             appName,
		"requested_token_type": "actor_access_token",
		"source_token_type":    "refresh_token",
		"source_device_tokens": []map[string]any{{
			"account_refresh_token": map[string]string{"token": refresh},
			"device_type":           DeviceType,
		}},
	}
	var out struct {
		Tokens []struct {
			Access struct {
				Token string `json:"token"`
			} `json:"actor_access_token"`
		} `json:"device_tokens"`
	}
	if err := post(ctx, hc, host+"/auth/token", body, &out); err != nil {
		return "", fmt.Errorf("prime video: actor token: %w", err)
	}
	if len(out.Tokens) == 0 || out.Tokens[0].Access.Token == "" {
		return "", errors.New("prime video: actor token: none issued")
	}
	return out.Tokens[0].Access.Token, nil
}

func post(ctx context.Context, hc *http.Client, url string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d: %.300s", resp.StatusCode, data)
	}
	return json.Unmarshal(data, out)
}
