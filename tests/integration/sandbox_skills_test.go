//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

func TestSkill_WithSandbox_MetadataParsed(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "skill-sandbox-meta", "pro")
	ctx := context.Background()

	// Upload a skill with sandbox: required metadata
	skillContent := `---
name: code-runner
description: Runs code in sandbox
version: "1.0"
sandbox: required
sandbox_tools: [read_file, write_file, terminal]
timeout: 60
---
# Code Runner
Execute arbitrary code safely.
`
	key := tenant.ID + "/code-runner/SKILL.md"
	if err := testEnv.MinIO.PutObject(ctx, key, []byte(skillContent)); err != nil {
		t.Fatalf("put skill: %v", err)
	}

	// Load via MinIO loader and verify metadata
	loader := skills.NewMinIOSkillLoader(testEnv.MinIO, tenant.ID)
	entry, err := loader.Find(ctx, "code-runner")
	if err != nil {
		t.Fatalf("find skill: %v", err)
	}

	if entry.Meta.Sandbox != "required" {
		t.Errorf("expected sandbox=required, got %q", entry.Meta.Sandbox)
	}
	if len(entry.Meta.SandboxTools) != 3 {
		t.Errorf("expected 3 sandbox_tools, got %d: %v", len(entry.Meta.SandboxTools), entry.Meta.SandboxTools)
	}
	if entry.Meta.Timeout != 60 {
		t.Errorf("expected timeout=60, got %d", entry.Meta.Timeout)
	}

	// Verify sandbox_tools contains expected values
	expected := map[string]bool{"read_file": true, "write_file": true, "terminal": true}
	for _, tool := range entry.Meta.SandboxTools {
		if !expected[tool] {
			t.Errorf("unexpected sandbox_tool: %q", tool)
		}
	}
}

func TestSkill_SandboxConfig_PerTenant(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sandbox-cfg-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sandbox-cfg-b", "enterprise")
	ctx := context.Background()

	// Set different sandbox policies via direct SQL
	policyA := store.SandboxPolicy{
		Enabled:         true,
		MaxTimeout:      30,
		AllowedTools:    []string{"read_file"},
		AllowDocker:     false,
		RestrictNetwork: true,
		MaxStdoutKB:     10,
	}
	policyB := store.SandboxPolicy{
		Enabled:         true,
		MaxTimeout:      120,
		AllowedTools:    []string{"read_file", "write_file", "terminal", "web_search"},
		AllowDocker:     true,
		RestrictNetwork: false,
		MaxStdoutKB:     100,
	}

	policyAJSON, _ := json.Marshal(policyA)
	policyBJSON, _ := json.Marshal(policyB)

	_, err := testEnv.Pool.Exec(ctx, "UPDATE tenants SET sandbox_policy = $1 WHERE id = $2", policyAJSON, tenantA.ID)
	if err != nil {
		t.Fatalf("set policy A: %v", err)
	}
	_, err = testEnv.Pool.Exec(ctx, "UPDATE tenants SET sandbox_policy = $1 WHERE id = $2", policyBJSON, tenantB.ID)
	if err != nil {
		t.Fatalf("set policy B: %v", err)
	}

	// Read back and verify policies are different
	var rawA, rawB []byte
	testEnv.Pool.QueryRow(ctx, "SELECT sandbox_policy FROM tenants WHERE id = $1", tenantA.ID).Scan(&rawA)
	testEnv.Pool.QueryRow(ctx, "SELECT sandbox_policy FROM tenants WHERE id = $1", tenantB.ID).Scan(&rawB)

	var readBackA, readBackB store.SandboxPolicy
	json.Unmarshal(rawA, &readBackA)
	json.Unmarshal(rawB, &readBackB)

	if readBackA.MaxTimeout != 30 {
		t.Errorf("tenant A max_timeout should be 30, got %d", readBackA.MaxTimeout)
	}
	if readBackB.MaxTimeout != 120 {
		t.Errorf("tenant B max_timeout should be 120, got %d", readBackB.MaxTimeout)
	}
	if readBackA.AllowDocker {
		t.Error("tenant A should not allow docker")
	}
	if !readBackB.AllowDocker {
		t.Error("tenant B should allow docker")
	}
	if !readBackA.RestrictNetwork {
		t.Error("tenant A should restrict network")
	}
	if readBackB.RestrictNetwork {
		t.Error("tenant B should not restrict network")
	}

	// Verify allowed tools enforcement
	cfgA := tools.SandboxConfig{
		AllowedTools: readBackA.AllowedTools,
		MaxToolCalls: tools.DefaultMaxToolCalls,
	}
	cfgB := tools.SandboxConfig{
		AllowedTools: readBackB.AllowedTools,
		MaxToolCalls: tools.DefaultMaxToolCalls,
	}

	// Tenant A can only use read_file
	if !cfgA.IsToolAllowed("read_file") {
		t.Error("tenant A should allow read_file")
	}
	if cfgA.IsToolAllowed("write_file") {
		t.Error("tenant A should NOT allow write_file")
	}

	// Tenant B has broader permissions
	if !cfgB.IsToolAllowed("write_file") {
		t.Error("tenant B should allow write_file")
	}
	if !cfgB.IsToolAllowed("web_search") {
		t.Error("tenant B should allow web_search")
	}
}

func TestSkill_SandboxRPC_ToolForwarding(t *testing.T) {
	cfg := tools.SandboxConfig{
		MaxToolCalls: 10,
		AllowedTools: []string{"read_file", "write_file"},
	}
	metrics := &tools.ExecMetrics{}

	// Create RPC request for an allowed tool
	rpcDir := t.TempDir()
	reqData := `{"tool_name":"read_file","args":{"path":"/tmp/nonexistent"}}`
	if err := writeRPCRequest(rpcDir, reqData); err != nil {
		t.Fatal(err)
	}

	processed := tools.ProcessToolCallRequest(rpcDir, &cfg, metrics)
	if !processed {
		t.Fatal("expected RPC request to be processed")
	}

	if metrics.ToolCallCount != 1 {
		t.Errorf("expected tool_call_count=1, got %d", metrics.ToolCallCount)
	}
}

func TestSkill_SandboxRPC_BlockedTool(t *testing.T) {
	cfg := tools.SandboxConfig{
		MaxToolCalls: 10,
		AllowedTools: []string{"read_file"},
	}
	metrics := &tools.ExecMetrics{}

	rpcDir := t.TempDir()
	reqData := `{"tool_name":"execute_command","args":{"cmd":"rm -rf /"}}`
	if err := writeRPCRequest(rpcDir, reqData); err != nil {
		t.Fatal(err)
	}

	processed := tools.ProcessToolCallRequest(rpcDir, &cfg, metrics)
	if !processed {
		t.Fatal("expected RPC request to be processed")
	}

	// Read response
	resp := readRPCResponse(t, rpcDir)
	if !strings.Contains(resp.Error, "not in sandbox allowlist") {
		t.Errorf("expected allowlist error, got: %q", resp.Error)
	}
}

func TestSkill_SandboxExecution_PerTenant_Isolation(t *testing.T) {
	tenantA := testEnv.CreateTestTenant(t, "sandbox-exec-a", "pro")
	tenantB := testEnv.CreateTestTenant(t, "sandbox-exec-b", "pro")

	// Execute code as tenant A
	resultA := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "python",
		"code":     "import os; print(os.environ.get('TENANT_ID', 'none'))",
		"timeout":  float64(10),
	}, &tools.ToolContext{TenantID: tenantA.ID})

	// Execute code as tenant B
	resultB := tools.Registry().Dispatch("execute_code", map[string]any{
		"language": "python",
		"code":     "import os; print(os.environ.get('TENANT_ID', 'none'))",
		"timeout":  float64(10),
	}, &tools.ToolContext{TenantID: tenantB.ID})

	// Neither should leak the other's identity (env is stripped)
	var respA, respB map[string]any
	json.Unmarshal([]byte(resultA), &respA)
	json.Unmarshal([]byte(resultB), &respB)

	stdoutA, _ := respA["stdout"].(string)
	stdoutB, _ := respB["stdout"].(string)

	// Since safeEnv() strips all custom vars, both should print "none"
	if strings.Contains(stdoutA, tenantB.ID) {
		t.Errorf("tenant A execution leaked tenant B's ID")
	}
	if strings.Contains(stdoutB, tenantA.ID) {
		t.Errorf("tenant B execution leaked tenant A's ID")
	}
}

func TestSkill_SandboxPolicy_NullDefault(t *testing.T) {
	tenant := testEnv.CreateTestTenant(t, "sandbox-null", "free")
	ctx := context.Background()

	// Default tenant should have NULL sandbox_policy
	var raw []byte
	err := testEnv.Pool.QueryRow(ctx, "SELECT sandbox_policy FROM tenants WHERE id = $1", tenant.ID).Scan(&raw)
	if err != nil {
		t.Fatalf("query sandbox_policy: %v", err)
	}

	if raw != nil {
		t.Errorf("new tenant should have NULL sandbox_policy, got: %s", string(raw))
	}
}

// --- helpers ---

func writeRPCRequest(rpcDir, reqJSON string) error {
	return os.WriteFile(rpcDir+"/request.json", []byte(reqJSON), 0644)
}

func readRPCResponse(t *testing.T, rpcDir string) tools.ToolCallResponse {
	t.Helper()
	data, err := os.ReadFile(rpcDir + "/response.json")
	if err != nil {
		t.Fatalf("read response.json: %v", err)
	}
	var resp tools.ToolCallResponse
	json.Unmarshal(data, &resp)
	return resp
}
