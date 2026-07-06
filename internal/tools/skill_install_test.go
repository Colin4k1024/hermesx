package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
)

type fakeSkillObjectStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeSkillObjectStore() *fakeSkillObjectStore {
	return &fakeSkillObjectStore{objects: map[string][]byte{}}
}

func (s *fakeSkillObjectStore) EnsureBucket(context.Context) error { return nil }
func (s *fakeSkillObjectStore) Bucket() string                     { return "test-bucket" }
func (s *fakeSkillObjectStore) Ping(context.Context) error         { return nil }
func (s *fakeSkillObjectStore) GetObject(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objects[key], nil
}
func (s *fakeSkillObjectStore) PutObject(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = append([]byte(nil), data...)
	return nil
}
func (s *fakeSkillObjectStore) PutObjectWithContentType(_ context.Context, key string, data []byte, _ string) error {
	return s.PutObject(context.Background(), key, data)
}
func (s *fakeSkillObjectStore) DeleteObject(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
func (s *fakeSkillObjectStore) ObjectExists(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objects[key]
	return ok, nil
}
func (s *fakeSkillObjectStore) ListObjects(_ context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for key := range s.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func TestSkillInstall_DirectURLUploadsToTenantObjectStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/frontend-design/SKILL.md" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("# Frontend Design\n\nUse polished UI patterns."))
	}))
	defer server.Close()

	store := newFakeSkillObjectStore()
	result := handleSkillInstall(context.Background(), map[string]any{
		"source_url": server.URL + "/frontend-design/SKILL.md",
		"skill":      "frontend-design",
	}, &ToolContext{
		TenantID:        "tenant-1",
		UserID:          "user-1",
		ObjectStore:     store,
		HTTPClient:      server.Client(),
		AllowPrivateIPs: true,
	})

	if !jsonContains(result, `"success":true`) {
		t.Fatalf("expected success response, got %s", result)
	}
	key := "tenant-1/frontend-design/SKILL.md"
	if string(store.objects[key]) != "# Frontend Design\n\nUse polished UI patterns." {
		t.Fatalf("expected uploaded SKILL.md at %s, objects=%#v", key, store.objects)
	}
}

func TestSkillInstall_ParsesSkillsAddCommand(t *testing.T) {
	source, skill := parseSkillAddCommand("npx skills add https://github.com/anthropics/skills --skill frontend-design")
	if source != "https://github.com/anthropics/skills" {
		t.Fatalf("source = %q", source)
	}
	if skill != "frontend-design" {
		t.Fatalf("skill = %q", skill)
	}
}

func jsonContains(s, sub string) bool {
	return len(s) >= len(sub) && containsString(s, sub)
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
