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

// SupermemoryProvider implements agent.MemoryProvider using the Supermemory API.
type SupermemoryProvider struct {
	baseURL string
	apiKey  string
	userID  string
	client  *http.Client
}

func NewSupermemoryProvider(userID string) *SupermemoryProvider {
	baseURL := os.Getenv("SUPERMEMORY_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.supermemory.ai/v1"
	}
	return &SupermemoryProvider{
		baseURL: baseURL,
		apiKey:  os.Getenv("SUPERMEMORY_API_KEY"),
		userID:  userID,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *SupermemoryProvider) ReadMemory() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/memories?userId=%s", s.baseURL, s.userID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func (s *SupermemoryProvider) SaveMemory(key, content string) error {
	payload := map[string]any{"content": content, "userId": s.userID, "metadata": map[string]string{"key": key}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.baseURL+"/memories", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supermemory error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *SupermemoryProvider) DeleteMemory(key string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/memories?userId=%s&key=%s", s.baseURL, s.userID, key), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *SupermemoryProvider) ReadUserProfile() (string, error) { return s.ReadMemory() }
func (s *SupermemoryProvider) SaveUserProfile(content string) error {
	return s.SaveMemory("user_profile", content)
}
