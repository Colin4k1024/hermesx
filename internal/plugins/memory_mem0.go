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

// Mem0MemoryProvider implements agent.MemoryProvider using the Mem0 API.
type Mem0MemoryProvider struct {
	baseURL string
	apiKey  string
	userID  string
	client  *http.Client
}

func NewMem0MemoryProvider(userID string) *Mem0MemoryProvider {
	baseURL := os.Getenv("MEM0_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.mem0.ai/v1"
	}
	return &Mem0MemoryProvider{
		baseURL: baseURL,
		apiKey:  os.Getenv("MEM0_API_KEY"),
		userID:  userID,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *Mem0MemoryProvider) ReadMemory() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/memories/?user_id=%s", m.baseURL, m.userID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func (m *Mem0MemoryProvider) SaveMemory(key, content string) error {
	payload := map[string]any{
		"messages": []map[string]string{{"role": "user", "content": content}},
		"user_id":  m.userID,
		"metadata": map[string]string{"key": key},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", m.baseURL+"/memories/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mem0 error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (m *Mem0MemoryProvider) DeleteMemory(key string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/memories/?user_id=%s", m.baseURL, m.userID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+m.apiKey)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (m *Mem0MemoryProvider) ReadUserProfile() (string, error) {
	return m.ReadMemory()
}

func (m *Mem0MemoryProvider) SaveUserProfile(content string) error {
	return m.SaveMemory("user_profile", content)
}
