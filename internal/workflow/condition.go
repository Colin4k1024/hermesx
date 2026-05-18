package workflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func conditionMatches(cond *store.WorkflowCondition, context map[string]any) (bool, error) {
	if cond == nil {
		return true, nil
	}
	value, exists := lookupPath(context, cond.Path)
	switch cond.Op {
	case "", "eq":
		if !exists {
			return false, nil
		}
		return normalizedEqual(value, cond.Value), nil
	case "ne":
		if !exists {
			return false, nil
		}
		return !normalizedEqual(value, cond.Value), nil
	case "exists":
		return exists, nil
	case "gt", "gte", "lt", "lte":
		if !exists {
			return false, nil
		}
		left, ok := toFloat(value)
		if !ok {
			return false, fmt.Errorf("condition path %q is not numeric", cond.Path)
		}
		right, ok := toFloat(cond.Value)
		if !ok {
			return false, fmt.Errorf("condition value for %q is not numeric", cond.Path)
		}
		switch cond.Op {
		case "gt":
			return left > right, nil
		case "gte":
			return left >= right, nil
		case "lt":
			return left < right, nil
		default:
			return left <= right, nil
		}
	case "contains":
		if !exists {
			return false, nil
		}
		switch v := value.(type) {
		case string:
			want, _ := cond.Value.(string)
			return strings.Contains(v, want), nil
		case []any:
			for _, item := range v {
				if normalizedEqual(item, cond.Value) {
					return true, nil
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("condition path %q does not support contains", cond.Path)
		}
	default:
		return false, fmt.Errorf("unsupported condition op %q", cond.Op)
	}
}

func lookupPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, part := range strings.Split(path, ".") {
		if strings.TrimSpace(part) == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = typed[part]
			if !ok {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return current, true
}

func normalizedEqual(a, b any) bool {
	if af, ok := toFloat(a); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
