package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/tools/environments"
)

func init() {
	Register(&ToolEntry{
		Name:    "execute_code",
		Toolset: "code_execution",
		Schema: map[string]any{
			"name":        "execute_code",
			"description": "Execute code in a sandboxed subprocess. Supports Python and Bash. Environment variables are stripped for security.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{
						"type":        "string",
						"description": "Programming language to execute",
						"enum":        []string{"python", "bash"},
					},
					"code": map[string]any{
						"type":        "string",
						"description": "Code to execute",
					},
					"timeout": map[string]any{
						"type":        "integer",
						"description": "Execution timeout in seconds (default: 30, max: 120)",
						"default":     30,
					},
				},
				"required": []string{"language", "code"},
			},
		},
		Handler: handleExecuteCode,
		Emoji:   "\u25b6\ufe0f",
	})
}

// safeEnv returns a minimal set of environment variables for sandboxed execution.
func safeEnv() []string {
	safe := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LANG=en_US.UTF-8",
		"TERM=xterm-256color",
	}
	// Include TMPDIR if set
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		safe = append(safe, "TMPDIR="+tmp)
	}
	return safe
}

func handleExecuteCode(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	language, _ := args["language"].(string)
	code, _ := args["code"].(string)

	if language == "" {
		return `{"error":"language is required"}`
	}
	if code == "" {
		return `{"error":"code is required"}`
	}

	cfg := DefaultSandboxConfig()

	timeout := 30
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = int(t)
	}
	maxTimeout := MaxTimeoutSec()
	if timeout > maxTimeout {
		timeout = maxTimeout
	}
	cfg.Timeout = time.Duration(timeout) * time.Second

	// Check SANDBOX_MODE to determine execution backend.
	sandboxMode := os.Getenv("SANDBOX_MODE")
	if sandboxMode == "" {
		return toJSON(map[string]any{
			"error": "SANDBOX_MODE is required for execute_code in SaaS mode",
			"hint":  "Set SANDBOX_MODE=docker for container isolation or SANDBOX_MODE=k8s-job for Kubernetes jobs. Local sandbox (SANDBOX_MODE=local) is only for development/testing.",
		})
	}

	switch sandboxMode {
	case "local":
		if !localSandboxAllowed() {
			return toJSON(map[string]any{
				"error": "local SANDBOX_MODE is disabled in SaaS-only mode",
				"hint":  "Local sandbox is ONLY for development/testing. For SaaS mode, use SANDBOX_MODE=docker or SANDBOX_MODE=k8s-job. To enable local sandbox for testing, set HERMESX_ALLOW_LOCAL_SANDBOX=true and ensure you are not in production/SaaS mode.",
			})
		}
		switch language {
		case "python":
			return executePython(ctx, code, &cfg)
		case "bash":
			return executeBash(ctx, code, &cfg)
		default:
			return toJSON(map[string]any{"error": fmt.Sprintf("Unsupported language: %s", language)})
		}

	case "docker", "k8s-job":
		// Route execution through the environments package.
		return executeViaEnvironment(ctx, sandboxMode, language, code, &cfg)

	default:
		return toJSON(map[string]any{"error": fmt.Sprintf("Unknown SANDBOX_MODE: %s (valid: local, docker, k8s-job)", sandboxMode)})
	}
}

// localSandboxAllowed checks if local sandbox execution is permitted.
// Local sandbox is ONLY allowed for development/testing when:
// 1. HERMESX_ALLOW_LOCAL_SANDBOX=true is explicitly set
// 2. NOT running in production environment
// 3. NOT running in SaaS mode (SAAS_API_PORT is not set)
func localSandboxAllowed() bool {
	if !envBool("HERMESX_ALLOW_LOCAL_SANDBOX") {
		return false
	}
	// Check production environment
	for _, name := range []string{"HERMES_ENV", "HERMESX_ENV", "APP_ENV", "GO_ENV"} {
		if strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "production") {
			return false
		}
	}
	// Check if running in SaaS mode
	if os.Getenv("SAAS_API_PORT") != "" {
		return false
	}
	return true
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func executePython(ctx context.Context, code string, cfg *SandboxConfig) string {
	// Write code to a temporary file
	tmpDir := filepath.Join(config.HermesHome(), "cache")
	os.MkdirAll(tmpDir, 0755)

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("exec_%d.py", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to write temp file: %v", err)})
	}
	defer os.Remove(tmpFile)

	// Create output directory for local sandbox
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(tmpDir, "output")
	}
	os.MkdirAll(outputDir, 0755)
	// Cleanup output directory on completion (timeout or error)
	defer cleanupOutputDir(outputDir)

	return runSandboxed(ctx, "python3", []string{tmpFile}, outputDir, "python", cfg)
}

func executeBash(ctx context.Context, code string, cfg *SandboxConfig) string {
	// Create output directory for local sandbox
	tmpDir := filepath.Join(config.HermesHome(), "cache")
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(tmpDir, "output")
	}
	os.MkdirAll(outputDir, 0755)
	// Cleanup output directory on completion (timeout or error)
	defer cleanupOutputDir(outputDir)

	return runSandboxed(ctx, "bash", []string{"-c", code}, outputDir, "bash", cfg)
}

// cleanupOutputDir removes all files in the output directory after execution.
// This ensures no temp files persist after timeout or error conditions.
func cleanupOutputDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}

// executeViaEnvironment routes code execution through the environments package,
// supporting docker and k8s-job backends selected via SANDBOX_MODE.
// Resource limits from SandboxConfig are forwarded as environment parameters.
func executeViaEnvironment(ctx context.Context, mode, language, code string, cfg *SandboxConfig) string {
	params := map[string]string{
		"memory_limit": fmt.Sprintf("%dMi", cfg.MemoryLimitMB),
		"cpu_limit":    cfg.CPULimit,
	}
	env, err := environments.GetEnvironment(mode, params)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to initialize %s environment: %v", mode, err)})
	}

	if !env.IsAvailable() {
		return toJSON(map[string]any{
			"error": fmt.Sprintf("Environment %q is not available (is the backend accessible?)", mode),
			"hint":  "Check that the required tooling (docker/kubectl) is installed and the backend is reachable.",
		})
	}

	// Build the command string based on language.
	// Output is restricted to cfg.OutputDir within the sandbox.
	var command string
	outputSetup := fmt.Sprintf("mkdir -p %s && ", cfg.OutputDir)
	switch language {
	case "python":
		// Pass code via heredoc to avoid quoting issues.
		command = outputSetup + fmt.Sprintf("cd %s && python3 -c %q", cfg.OutputDir, code)
	case "bash":
		command = outputSetup + fmt.Sprintf("cd %s && %s", cfg.OutputDir, code)
	default:
		return toJSON(map[string]any{"error": fmt.Sprintf("Unsupported language: %s", language)})
	}

	startTime := time.Now()
	stdout, stderr, exitCode, execErr := env.Execute(command, int(cfg.Timeout.Seconds()))
	wallTime := time.Since(startTime).Milliseconds()

	metrics := ExecMetrics{
		WallTimeMs:  wallTime,
		ExitCode:    exitCode,
		StdoutBytes: len(stdout),
		StderrBytes: len(stderr),
	}

	if execErr != nil {
		slog.Info("sandbox environment execution error",
			"mode", mode, "language", language, "error", execErr)
		return toJSON(map[string]any{
			"error":     execErr.Error(),
			"stdout":    stdout,
			"stderr":    stderr,
			"exit_code": exitCode,
			"metrics":   metrics,
		})
	}

	return toJSON(map[string]any{
		"stdout":      stdout,
		"stderr":      stderr,
		"exit_code":   exitCode,
		"language":    language,
		"duration_ms": wallTime,
		"metrics":     metrics,
	})
}

// runSandboxed executes a command inside the sandbox constraints defined by cfg.
func runSandboxed(ctx context.Context, bin string, cmdArgs []string, workDir, language string, cfg *SandboxConfig) string {
	execCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, bin, cmdArgs...)
	cmd.Env = safeEnv()
	cmd.Dir = workDir

	stdoutW := NewLimitedWriter(cfg.MaxStdoutBytes)
	stderrW := NewLimitedWriter(cfg.MaxStderrBytes)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	metrics := ExecMetrics{}
	startTime := time.Now()
	err := cmd.Run()
	metrics.WallTimeMs = time.Since(startTime).Milliseconds()
	metrics.StdoutBytes = stdoutW.Len()
	metrics.StderrBytes = stderrW.Len()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			metrics.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			metrics.ExitCode = -1
			slog.Info("sandbox execution timed out", "language", language, "timeout", cfg.Timeout)
			return toJSON(map[string]any{
				"error":     "Execution timed out",
				"timeout":   int(cfg.Timeout.Seconds()),
				"stdout":    stdoutW.String(),
				"stderr":    stderrW.String(),
				"exit_code": metrics.ExitCode,
				"metrics":   metrics,
			})
		}
	}

	return toJSON(map[string]any{
		"stdout":      stdoutW.String(),
		"stderr":      stderrW.String(),
		"exit_code":   metrics.ExitCode,
		"language":    language,
		"duration_ms": metrics.WallTimeMs,
		"metrics":     metrics,
	})
}
