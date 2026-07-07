package tools

import (
	"bytes"
	"context"
	"encoding/base64"
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

const (
	// MaxDocGenFileSize limits the generated file to 50MB.
	MaxDocGenFileSize = 50 * 1024 * 1024
	// DocGenTimeout is the max timeout for document generation sandbox.
	DocGenTimeout = 120 * time.Second
	// DocGenOutputDir is the output directory inside the sandbox.
	DocGenOutputDir = "/tmp/output"
)

// executePythonForFile runs a Python script in the sandbox and returns the bytes
// of the file written to outputPath inside the sandbox. The sandboxMode determines
// the execution backend (local, docker, k8s-job).
// stdinData, if non-nil, is piped to the Python process's stdin. This allows
// passing user-supplied data without embedding it in the script source code.
func executePythonForFile(ctx context.Context, script string, outputPath string, sandboxMode string, stdinData []byte) ([]byte, error) {
	cfg := DefaultSandboxConfig()
	cfg.Timeout = DocGenTimeout
	cfg.OutputDir = DocGenOutputDir

	switch sandboxMode {
	case "local":
		if !localSandboxAllowed() {
			return nil, fmt.Errorf("local SANDBOX_MODE is disabled; use docker or k8s-job")
		}
		return executePythonForFileLocal(ctx, script, outputPath, &cfg, stdinData)

	case "docker":
		return executePythonForFileDocker(ctx, script, outputPath, &cfg, stdinData)

	case "k8s-job":
		return executePythonForFileK8s(ctx, script, outputPath, &cfg, stdinData)

	default:
		return nil, fmt.Errorf("unknown SANDBOX_MODE: %s (valid: local, docker, k8s-job)", sandboxMode)
	}
}

// executePythonForFileLocal runs the Python script locally and reads the output file.
func executePythonForFileLocal(ctx context.Context, script string, outputPath string, cfg *SandboxConfig, stdinData []byte) ([]byte, error) {
	tmpDir := filepath.Join(config.HermesHome(), "cache")
	os.MkdirAll(tmpDir, 0755)

	// Write script to temp file
	scriptFile := filepath.Join(tmpDir, fmt.Sprintf("docgen_%d.py", time.Now().UnixNano()))
	if err := os.WriteFile(scriptFile, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	defer os.Remove(scriptFile)

	// Create output directory
	os.MkdirAll(cfg.OutputDir, 0755)
	defer cleanupOutputDir(cfg.OutputDir)

	execCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "python3", scriptFile)
	cmd.Env = safeEnv()
	cmd.Dir = cfg.OutputDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	if len(stdinData) > 0 {
		cmd.Stdin = bytes.NewReader(stdinData)
	}

	if err := cmd.Run(); err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("python execution timed out after %v", cfg.Timeout)
		}
		return nil, fmt.Errorf("python execution failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	// Read the output file
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("output file not found at %s: %w (stderr: %s)", outputPath, err, stderrBuf.String())
	}
	return data, nil
}

// executePythonForFileDocker runs the Python script in a Docker container.
// It creates a fresh container with a volume-mounted output directory so that
// the generated file can be read from the host filesystem afterward.
func executePythonForFileDocker(ctx context.Context, script string, outputPath string, cfg *SandboxConfig, stdinData []byte) ([]byte, error) {
	// Create a host tmpdir that will be mounted as the output directory
	hostOutputDir, err := os.MkdirTemp("", "hermesx-docgen-")
	if err != nil {
		return nil, fmt.Errorf("failed to create host output dir: %w", err)
	}
	defer os.RemoveAll(hostOutputDir)

	// Write the Python script and stdin data to the host tmpdir
	scriptFile := filepath.Join(hostOutputDir, "generate.py")
	if err := os.WriteFile(scriptFile, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	if len(stdinData) > 0 {
		stdinFile := filepath.Join(hostOutputDir, "stdin_data.json")
		if err := os.WriteFile(stdinFile, stdinData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write stdin data: %w", err)
		}
	}

	// Determine container image
	image := os.Getenv("SANDBOX_DOCKER_IMAGE")
	if image == "" {
		image = "python:3.12-slim"
	}

	// The filename inside the container is derived from the output path
	_, filename := filepath.Split(outputPath)

	// Run a fresh container with the output dir mounted.
	// The Python script writes to /tmp/output/{filename}, which maps to hostOutputDir.
	// A wrapper installs python-docx/python-pptx before running the script.
	execCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	wrapperScript := fmt.Sprintf(`
import subprocess, sys, os
subprocess.check_call([sys.executable, "-m", "pip", "install", "-q", "python-docx", "python-pptx"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
stdin_path = "/mnt/stdin_data.json"
stdin_data = open(stdin_path, "rb").read() if os.path.exists(stdin_path) else None
proc = subprocess.run([sys.executable, "/mnt/generate.py"], input=stdin_data)
sys.exit(proc.returncode)
`)

	dockerArgs := []string{"run", "--rm",
		"-v", hostOutputDir + ":/mnt",
		"-v", hostOutputDir + ":" + DocGenOutputDir,
		"--network=none",
		"--memory", fmt.Sprintf("%dm", cfg.MemoryLimitMB),
		"--cpus", cfg.CPULimit,
		image,
		"python3", "-c", wrapperScript,
	}

	cmd := exec.CommandContext(execCtx, "docker", dockerArgs...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("docker execution timed out after %v", cfg.Timeout)
		}
		return nil, fmt.Errorf("docker execution failed: %w (stdout: %s, stderr: %s)", err, stdoutBuf.String(), stderrBuf.String())
	}

	// Read the output file from the host filesystem
	outputOnHost := filepath.Join(hostOutputDir, filename)
	data, err := os.ReadFile(outputOnHost)
	if err != nil {
		return nil, fmt.Errorf("output file not found at %s: %w (stderr: %s)", outputOnHost, err, stderrBuf.String())
	}
	return data, nil
}

// executePythonForFileK8s runs the Python script in a K8s Job.
// Because k8s jobs don't support direct file retrieval, the Python script
// encodes the output file as base64 to stdout using a marker protocol:
//
//	__FILE_BASE64_START__
//	<base64 data>
//	__FILE_BASE64_END__
func executePythonForFileK8s(ctx context.Context, script string, outputPath string, cfg *SandboxConfig, stdinData []byte) ([]byte, error) {
	params := map[string]string{
		"memory_limit": fmt.Sprintf("%dMi", cfg.MemoryLimitMB),
		"cpu_limit":    cfg.CPULimit,
	}
	env, err := environments.GetEnvironment("k8s-job", params)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize k8s-job environment: %w", err)
	}
	if !env.IsAvailable() {
		return nil, fmt.Errorf("k8s-job environment is not available")
	}

	// Wrap the script to output the file as base64 after generation.
	// If stdin data is provided, encode it as base64 and pass as a command-line
	// argument to avoid embedding user text in the script source.
	_, filename := filepath.Split(outputPath)
	_ = filename
	stdinB64 := ""
	if len(stdinData) > 0 {
		stdinB64 = base64.StdEncoding.EncodeToString(stdinData)
	}
	wrappedScript := fmt.Sprintf(`import base64, sys, os
_stdin_b64 = %q
if _stdin_b64:
    _data = base64.b64decode(_stdin_b64)
else:
    _data = None
import subprocess
_proc = subprocess.run([sys.executable, "-c", %q], input=_data)
if _proc.returncode != 0:
    sys.exit(_proc.returncode)
import base64
output_path = %q
if os.path.exists(output_path):
    with open(output_path, 'rb') as f:
        data = f.read()
    print("__FILE_BASE64_START__")
    print(base64.b64encode(data).decode('ascii'))
    print("__FILE_BASE64_END__")
else:
    print("__FILE_BASE64_ERROR__File not found: " + output_path, file=sys.stderr)
    exit(1)
`, stdinB64, script, outputPath)

	stdout, stderr, exitCode, execErr := env.Execute(wrappedScript, int(cfg.Timeout.Seconds()))
	if execErr != nil {
		return nil, fmt.Errorf("k8s-job execution failed: %w (stderr: %s)", execErr, stderr)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("k8s-job exited with code %d (stderr: %s)", exitCode, stderr)
	}

	// Parse base64 from stdout
	data, err := extractBase64FromOutput(stdout)
	if err != nil {
		slog.Error("docgen: failed to parse k8s-job output", "error", err)
		return nil, fmt.Errorf("failed to extract file from k8s-job output: %w", err)
	}
	return data, nil
}

// extractBase64FromOutput extracts the base64-encoded file data between markers.
func extractBase64FromOutput(output string) ([]byte, error) {
	startMarker := "__FILE_BASE64_START__"
	endMarker := "__FILE_BASE64_END__"

	startIdx := strings.Index(output, startMarker)
	if startIdx < 0 {
		return nil, fmt.Errorf("start marker not found in output")
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(output[startIdx:], endMarker)
	if endIdx < 0 {
		return nil, fmt.Errorf("end marker not found in output")
	}

	b64Data := strings.TrimSpace(output[startIdx : startIdx+endIdx])
	decoded, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}
	return decoded, nil
}
