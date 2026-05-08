package api

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// memoryExtractor extracts user facts from messages and persists them via store.MemoryStore.
// Works with any store backend (MySQL, PostgreSQL, SQLite).
type memoryExtractor struct {
	memStore store.MemoryStore
}

type extractedMemory struct {
	Key     string
	Content string
}

var (
	reRememberEN = regexp.MustCompile(`(?i)remember[:\s]+(.+)`)
	reRememberZH = regexp.MustCompile(`(?:记住|请记住)[：:\s]+(.+)`)
	reMyNameEN   = regexp.MustCompile(`(?i)my name is\s+(\S+(?:\s+\S+)?)`)
	reMyNameZH   = regexp.MustCompile(`我(?:的名字)?叫\s*([^\s，,。！？、]+)`)
	reMyXIsYEN   = regexp.MustCompile(`(?i)my\s+(favorite\s+\w+|name|age|job|city|email|phone|hobby|language)\s+is\s+(.+?)(?:\.|,|!|\?|$)`)
	reMyXIsYZH   = regexp.MustCompile(`我(?:最喜欢的|的)\s*(\S+?)\s*(?:是|为)\s*(.+?)(?:[。，！？\s]|$)`)
	reFavEN      = regexp.MustCompile(`(?i)(?:i (?:like|love|prefer|enjoy))\s+(.+?)(?:\s+(?:the most|a lot|very much))?(?:\.|,|!|\?|$)`)
	reCallMeEN   = regexp.MustCompile(`(?i)(?:call me|you can call me)\s+(\S+)`)

	reIdentityZH   = regexp.MustCompile(`我(?:的身份|的职业|的工作)(?:是|为)\s*(.+?)(?:[。，！？\s]|$)`)
	reIAmAEN       = regexp.MustCompile(`(?i)i\s+am\s+(?:a |an )?(\w+(?:\s+\w+){0,3})(?:\.|,|!|\?|$)`)
	reProfessionZH = regexp.MustCompile(`我是(?:一个|一名|一位)?\s*(.+?)(?:[。，！？\s]|$)`)
	reFavZH        = regexp.MustCompile(`我(?:喜欢|偏好|爱好)\s*(.+?)(?:[。，！？\s]|$)`)
)

func (e *memoryExtractor) extract(userMessage string) []extractedMemory {
	var results []extractedMemory

	if m := reRememberEN.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" {
			results = append(results, extractedMemory{Key: deriveKey(content), Content: content})
		}
	}
	if m := reRememberZH.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" {
			results = append(results, extractedMemory{Key: deriveKey(content), Content: content})
		}
	}

	if m := reMyNameEN.FindStringSubmatch(userMessage); len(m) > 1 {
		results = append(results, extractedMemory{Key: "user_name", Content: strings.TrimSpace(m[1])})
	}
	if m := reMyNameZH.FindStringSubmatch(userMessage); len(m) > 1 {
		results = append(results, extractedMemory{Key: "user_name", Content: strings.TrimSpace(m[1])})
	}
	if m := reCallMeEN.FindStringSubmatch(userMessage); len(m) > 1 {
		results = append(results, extractedMemory{Key: "user_name", Content: strings.TrimSpace(m[1])})
	}

	if m := reMyXIsYEN.FindStringSubmatch(userMessage); len(m) > 2 {
		key := normalizeKey(m[1])
		content := strings.TrimSpace(m[2])
		if content != "" {
			results = append(results, extractedMemory{Key: key, Content: content})
		}
	}
	if m := reMyXIsYZH.FindStringSubmatch(userMessage); len(m) > 2 {
		key := normalizeKey(m[1])
		content := strings.TrimSpace(m[2])
		if content != "" {
			results = append(results, extractedMemory{Key: key, Content: content})
		}
	}

	if m := reFavEN.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" && !strings.Contains(strings.ToLower(content), "you") {
			results = append(results, extractedMemory{Key: "preference", Content: "likes " + content})
		}
	}

	if m := reIdentityZH.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" {
			results = append(results, extractedMemory{Key: "profession", Content: content})
		}
	}

	if m := reIAmAEN.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" && len(content) < 50 {
			results = append(results, extractedMemory{Key: "identity", Content: content})
		}
	}

	if m := reProfessionZH.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" && len(content) < 30 && !strings.Contains(content, "谁") {
			results = append(results, extractedMemory{Key: "identity", Content: content})
		}
	}

	if m := reFavZH.FindStringSubmatch(userMessage); len(m) > 1 {
		content := strings.TrimSpace(m[1])
		if content != "" {
			results = append(results, extractedMemory{Key: "preference_zh", Content: "喜欢" + content})
		}
	}

	return dedup(results)
}

func (e *memoryExtractor) persist(tenantID, userID string, memories []extractedMemory) {
	if e.memStore == nil || len(memories) == 0 {
		return
	}
	ctx, cancel := newCtx5s()
	defer cancel()
	for _, m := range memories {
		if err := e.memStore.Upsert(ctx, tenantID, userID, m.Key, m.Content); err != nil {
			slog.Warn("memory_extractor: failed to save", "key", m.Key, "error", err)
		} else {
			slog.Info("memory_extracted", "tenant", tenantID, "user", userID, "key", m.Key, "content", m.Content)
		}
	}
}

func deriveKey(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "fruit"):
		return "favorite_fruit"
	case strings.Contains(lower, "city") || strings.Contains(lower, "cities"):
		return "favorite_city"
	case strings.Contains(lower, "color") || strings.Contains(lower, "colour"):
		return "favorite_color"
	case strings.Contains(lower, "food") || strings.Contains(lower, "dish"):
		return "favorite_food"
	case strings.Contains(lower, "music") || strings.Contains(lower, "song"):
		return "favorite_music"
	case strings.Contains(lower, "movie") || strings.Contains(lower, "film"):
		return "favorite_movie"
	case strings.Contains(lower, "book"):
		return "favorite_book"
	case strings.Contains(lower, "sport"):
		return "favorite_sport"
	case strings.Contains(lower, "name"):
		return "user_name"
	default:
		key := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
				return r
			}
			if r == ' ' || r == '-' {
				return '_'
			}
			return -1
		}, lower)
		if len(key) > 40 {
			key = key[:40]
		}
		if key == "" {
			key = "note"
		}
		return "remembered_" + key
	}
}

func normalizeKey(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	lower = strings.ReplaceAll(lower, " ", "_")
	return lower
}

func dedup(memories []extractedMemory) []extractedMemory {
	seen := make(map[string]bool)
	var result []extractedMemory
	for _, m := range memories {
		if seen[m.Key] {
			continue
		}
		seen[m.Key] = true
		result = append(result, m)
	}
	return result
}

func newCtx5s() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
