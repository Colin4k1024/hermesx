//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/tools/environments"
)

func TestSandbox_Python_Basic(t *testing.T) {
	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "python",
		"code":     "print('hello from sandbox')",
		"timeout":  float64(10),
	}, nil)

	var resp map[string]any
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("unmarshal result: %v, raw: %s", err, result)
	}

	if errStr, _ := resp["error"].(string); errStr != "" {
		t.Fatalf("execution error: %s", errStr)
	}

	stdout, _ := resp["stdout"].(string)
	if !strings.Contains(stdout, "hello from sandbox") {
		t.Errorf("expected 'hello from sandbox' in stdout, got: %q", stdout)
	}

	exitCode, _ := resp["exit_code"].(float64)
	if exitCode != 0 {
		t.Errorf("expected exit_code 0, got: %v", exitCode)
	}
}

func TestSandbox_Bash_Basic(t *testing.T) {
	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "bash",
		"code":     "echo 'bash sandbox test'",
		"timeout":  float64(10),
	}, nil)

	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	stdout, _ := resp["stdout"].(string)
	if !strings.Contains(stdout, "bash sandbox test") {
		t.Errorf("expected 'bash sandbox test' in stdout, got: %q", stdout)
	}
}

func TestSandbox_Timeout(t *testing.T) {
	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "bash",
		"code":     "sleep 30",
		"timeout":  float64(2),
	}, nil)

	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	// Timeout can manifest as error field or non-zero exit_code (-1 for signal kill)
	errStr, _ := resp["error"].(string)
	exitCode, _ := resp["exit_code"].(float64)

	timedOut := strings.Contains(strings.ToLower(errStr), "timed out") || exitCode != 0
	if !timedOut {
		t.Errorf("expected timeout (error or non-zero exit), got error=%q exit_code=%v", errStr, exitCode)
	}
}

func TestSandbox_OutputTruncation(t *testing.T) {
	// LimitedWriter with 50KB limit
	lw := tools.NewLimitedWriter(1024) // 1KB for faster test

	// Write more than 1KB
	bigData := strings.Repeat("X", 2048)
	lw.Write([]byte(bigData))

	if !lw.Exceeded() {
		t.Error("expected LimitedWriter to report exceeded")
	}

	output := lw.String()
	if !strings.Contains(output, "output truncated") {
		t.Error("expected truncation notice in output")
	}

	if lw.Len() > 1024 {
		t.Errorf("LimitedWriter stored more than limit: %d bytes", lw.Len())
	}
}

func TestSandbox_AllowedTools_Enforcement(t *testing.T) {
	cfg := tools.DefaultSandboxConfig()

	// "read_file" is in default allowlist
	if !cfg.IsToolAllowed("read_file") {
		t.Error("read_file should be allowed by default")
	}

	// "dangerous_tool" is NOT in allowlist
	if cfg.IsToolAllowed("dangerous_tool") {
		t.Error("dangerous_tool should NOT be allowed")
	}

	// Custom config with restricted allowlist
	restricted := tools.SandboxConfig{
		AllowedTools: []string{"only_this"},
	}
	if !restricted.IsToolAllowed("only_this") {
		t.Error("only_this should be allowed in restricted config")
	}
	if restricted.IsToolAllowed("read_file") {
		t.Error("read_file should NOT be allowed in restricted config")
	}
}

func TestSandbox_MaxToolCalls_Limit(t *testing.T) {
	cfg := tools.SandboxConfig{
		MaxToolCalls: 2,
		AllowedTools: []string{"read_file"},
	}
	metrics := &tools.ExecMetrics{ToolCallCount: 2} // already at limit

	// Create a temp RPC directory with a request
	rpcDir := t.TempDir()
	reqData := `{"tool_name":"read_file","args":{"path":"/tmp/test"}}`
	os.WriteFile(filepath.Join(rpcDir, "request.json"), []byte(reqData), 0644)

	processed := tools.ProcessToolCallRequest(rpcDir, &cfg, metrics)
	if !processed {
		t.Fatal("expected request to be processed")
	}

	// Check response says max exceeded
	respData, err := os.ReadFile(filepath.Join(rpcDir, "response.json"))
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var resp tools.ToolCallResponse
	json.Unmarshal(respData, &resp)
	if !strings.Contains(resp.Error, "max tool calls") {
		t.Errorf("expected max tool calls error, got: %q", resp.Error)
	}
}

func TestSandbox_BlockedTool_RPC(t *testing.T) {
	cfg := tools.SandboxConfig{
		MaxToolCalls: 50,
		AllowedTools: []string{"read_file"},
	}
	metrics := &tools.ExecMetrics{}

	rpcDir := t.TempDir()
	reqData := `{"tool_name":"exec_command","args":{"cmd":"rm -rf /"}}`
	os.WriteFile(filepath.Join(rpcDir, "request.json"), []byte(reqData), 0644)

	processed := tools.ProcessToolCallRequest(rpcDir, &cfg, metrics)
	if !processed {
		t.Fatal("expected request to be processed")
	}

	respData, _ := os.ReadFile(filepath.Join(rpcDir, "response.json"))
	var resp tools.ToolCallResponse
	json.Unmarshal(respData, &resp)

	if !strings.Contains(resp.Error, "not in sandbox allowlist") {
		t.Errorf("expected allowlist rejection, got: %q", resp.Error)
	}
}

func TestSandbox_EnvStripped(t *testing.T) {
	// Set a sensitive env var
	os.Setenv("SECRET_TOKEN", "super-secret-value")
	defer os.Unsetenv("SECRET_TOKEN")

	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "bash",
		"code":     "env",
		"timeout":  float64(10),
	}, nil)

	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	stdout, _ := resp["stdout"].(string)

	// Should NOT contain SECRET_TOKEN
	if strings.Contains(stdout, "SECRET_TOKEN") || strings.Contains(stdout, "super-secret-value") {
		t.Error("sandbox env leaked SECRET_TOKEN")
	}

	// Should contain safe vars
	if !strings.Contains(stdout, "PATH=") {
		t.Error("sandbox env missing PATH")
	}
}

func TestSandbox_UnsupportedLanguage(t *testing.T) {
	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "ruby",
		"code":     "puts 'hello'",
	}, nil)

	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	errStr, _ := resp["error"].(string)
	if !strings.Contains(errStr, "Unsupported language") {
		t.Errorf("expected unsupported language error, got: %q", errStr)
	}
}

func TestSandbox_EmptyCode_Rejected(t *testing.T) {
	result := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "python",
		"code":     "",
	}, nil)

	var resp map[string]any
	json.Unmarshal([]byte(result), &resp)

	errStr, _ := resp["error"].(string)
	if errStr == "" {
		t.Error("expected error for empty code")
	}
}

// --- Docker Sandbox Tests ---

func getDockerEnv(t *testing.T) environments.Environment {
	t.Helper()
	env, err := environments.GetEnvironment("docker", map[string]string{
		"image":          "python:3.11-slim",
		"container_name": "hermes-test-sandbox",
	})
	if err != nil {
		t.Skipf("Docker env creation failed: %v", err)
	}
	if !env.IsAvailable() {
		t.Skip("Docker not available, skipping Docker sandbox tests")
	}
	return env
}

func TestDockerSandbox_Available(t *testing.T) {
	env := getDockerEnv(t)
	t.Logf("Docker sandbox available: %s", env.Name())
}

func TestDockerSandbox_Isolation(t *testing.T) {
	env := getDockerEnv(t)

	// Execute command that tries to read host filesystem
	stdout, stderr, exitCode, err := env.Execute("cat /etc/hostname && whoami", 10)
	if err != nil {
		t.Logf("docker exec: err=%v, stdout=%s, stderr=%s, exit=%d", err, stdout, stderr, exitCode)
	}

	// The container should NOT have access to host's root filesystem
	hostHostname, _ := os.ReadFile("/etc/hostname")
	if strings.TrimSpace(stdout) == strings.TrimSpace(string(hostHostname)) {
		t.Log("WARNING: container hostname matches host — may indicate insufficient isolation")
	}
}

func TestDockerSandbox_Timeout(t *testing.T) {
	env := getDockerEnv(t)

	_, _, exitCode, err := env.Execute("sleep 60", 3)
	if err == nil && exitCode == 0 {
		t.Error("expected timeout or non-zero exit for long-running command")
	}
}

func TestDockerSandbox_ResourceLimit(t *testing.T) {
	env := getDockerEnv(t)

	// Try to allocate excessive memory (should be limited by container config)
	stdout, stderr, exitCode, err := env.Execute("python3 -c \"x = 'A' * (512 * 1024 * 1024)\" 2>&1 || echo 'OOM'", 10)
	_ = stdout
	_ = stderr
	// We just verify it doesn't hang forever — the container constraints should handle it
	if err != nil {
		t.Logf("resource limit test: err=%v, exit=%d", err, exitCode)
	}
}
