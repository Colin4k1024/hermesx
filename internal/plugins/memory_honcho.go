package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HonchoMemoryProvider implements agent.MemoryProvider using the Honcho API.
type HonchoMemoryProvider struct {
	baseURL string
	apiKey  string
	appID   string
	userID  string
	client  *http.Client
}

// NewHonchoMemoryProvider creates a Honcho-backed memory provider.
func NewHonchoMemoryProvider(userID string) *HonchoMemoryProvider {
	baseURL := os.Getenv("HONCHO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.honcho.dev"
	}
	return &HonchoMemoryProvider{
		baseURL: baseURL,
		apiKey:  os.Getenv("HONCHO_API_KEY"),
		appID:   os.Getenv("HONCHO_APP_ID"),
		userID:  userID,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *HonchoMemoryProvider) ReadMemory() (string, error) {
	return h.apiGet(fmt.Sprintf("/apps/%s/users/%s/metamessages", h.appID, h.userID))
}

func (h *HonchoMemoryProvider) SaveMemory(key, content string) error {
	payload := map[string]any{"key": key, "content": content, "metamessage_type": "memory"}
	return h.apiPost(fmt.Sprintf("/apps/%s/users/%s/metamessages", h.appID, h.userID), payload)
}

func (h *HonchoMemoryProvider) DeleteMemory(key string) error {
	return h.apiPost(fmt.Sprintf("/apps/%s/users/%s/metamessages/delete", h.appID, h.userID), map[string]string{"key": key})
}

func (h *HonchoMemoryProvider) ReadUserProfile() (string, error) {
	return h.apiGet(fmt.Sprintf("/apps/%s/users/%s", h.appID, h.userID))
}

func (h *HonchoMemoryProvider) SaveUserProfile(content string) error {
	return h.apiPost(fmt.Sprintf("/apps/%s/users/%s", h.appID, h.userID), map[string]string{"metadata": content})
}

func (h *HonchoMemoryProvider) apiGet(path string) (string, error) {
	req, err := http.NewRequest("GET", h.baseURL+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *HonchoMemoryProvider) apiPost(path string, payload any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", h.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("honcho API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
