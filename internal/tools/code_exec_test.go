package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func decodeExecuteCodeResult(t *testing.T, raw string) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal result: %v, raw: %s", err, raw)
	}
	return resp
}

func TestExecuteCodeRequiresSandboxMode(t *testing.T) {
	t.Setenv("SANDBOX_MODE", "")
	t.Setenv("HERMESX_ALLOW_LOCAL_SANDBOX", "")

	result := handleExecuteCode(context.Background(), map[string]any{
		"language": "bash",
		"code":     "echo should-not-run",
	}, nil)

	resp := decodeExecuteCodeResult(t, result)
	errStr, _ := resp["error"].(string)
	if !strings.Contains(errStr, "SANDBOX_MODE is required") {
		t.Fatalf("error = %q, want explicit SANDBOX_MODE requirement", errStr)
	}
}

func TestExecuteCodeLocalRequiresExplicitDevelopmentOptIn(t *testing.T) {
	t.Setenv("SANDBOX_MODE", "local")
	t.Setenv("HERMESX_ALLOW_LOCAL_SANDBOX", "")
	t.Setenv("HERMES_ENV", "development")

	result := handleExecuteCode(context.Background(), map[string]any{
		"language": "bash",
		"code":     "echo should-not-run",
	}, nil)

	resp := decodeExecuteCodeResult(t, result)
	errStr, _ := resp["error"].(string)
	if !strings.Contains(errStr, "local SANDBOX_MODE is disabled") {
		t.Fatalf("error = %q, want local sandbox disabled message", errStr)
	}
}

func TestExecuteCodeLocalBlockedInProduction(t *testing.T) {
	t.Setenv("SANDBOX_MODE", "local")
	t.Setenv("HERMESX_ALLOW_LOCAL_SANDBOX", "true")
	t.Setenv("HERMES_ENV", "production")

	result := handleExecuteCode(context.Background(), map[string]any{
		"language": "bash",
		"code":     "echo should-not-run",
	}, nil)

	resp := decodeExecuteCodeResult(t, result)
	errStr, _ := resp["error"].(string)
	if !strings.Contains(errStr, "local SANDBOX_MODE is disabled") {
		t.Fatalf("error = %q, want production local sandbox rejection", errStr)
	}
}

func TestExecuteCodeLocalAllowedForExplicitDevelopment(t *testing.T) {
	t.Setenv("SANDBOX_MODE", "local")
	t.Setenv("HERMESX_ALLOW_LOCAL_SANDBOX", "true")
	t.Setenv("HERMES_ENV", "development")

	result := handleExecuteCode(context.Background(), map[string]any{
		"language": "bash",
		"code":     "printf ok",
	}, nil)

	resp := decodeExecuteCodeResult(t, result)
	if errStr, _ := resp["error"].(string); errStr != "" {
		t.Fatalf("execution error: %s", errStr)
	}
	stdout, _ := resp["stdout"].(string)
	if stdout != "ok" {
		t.Fatalf("stdout = %q, want ok", stdout)
	}
}
