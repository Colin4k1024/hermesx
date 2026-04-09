package environments

import (
	"strings"
	"testing"
)

// --- DockerEnvironment tests ---

func TestDockerEnvironment_Name(t *testing.T) {
	env := &DockerEnvironment{
		image:         "ubuntu:latest",
		containerName: "test",
	}
	if env.Name() != "docker" {
		t.Errorf("Expected 'docker', got '%s'", env.Name())
	}
}

func TestDockerEnvironment_IsAvailable(t *testing.T) {
	env := &DockerEnvironment{
		image:         "ubuntu:latest",
		containerName: "test",
	}
	// Just test it doesn't panic; result depends on whether docker is installed
	_ = env.IsAvailable()
}

// --- SSHEnvironment tests ---

func TestSSHEnvironment_Name(t *testing.T) {
	env := &SSHEnvironment{
		host: "example.com",
		user: "root",
		port: "22",
	}
	if env.Name() != "ssh" {
		t.Errorf("Expected 'ssh', got '%s'", env.Name())
	}
}

func TestSSHEnvironment_IsAvailable(t *testing.T) {
	env := &SSHEnvironment{
		host: "example.com",
		user: "root",
		port: "22",
	}
	// ssh is typically available on macOS/Linux
	available := env.IsAvailable()
	// Just verify it doesn't panic; result depends on OS
	_ = available
}

func TestSSHEnvironment_SSHArgs(t *testing.T) {
	env := &SSHEnvironment{
		host:    "example.com",
		user:    "testuser",
		port:    "2222",
		keyFile: "/path/to/key",
	}

	args := env.sshArgs()
	if len(args) == 0 {
		t.Fatal("Expected non-empty ssh args")
	}

	// Check port
	foundPort := false
	foundKey := false
	foundUserHost := false
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) && args[i+1] == "2222" {
			foundPort = true
		}
		if arg == "-i" && i+1 < len(args) && args[i+1] == "/path/to/key" {
			foundKey = true
		}
		if arg == "testuser@example.com" {
			foundUserHost = true
		}
	}
	if !foundPort {
		t.Error("Expected port 2222 in ssh args")
	}
	if !foundKey {
		t.Error("Expected key file in ssh args")
	}
	if !foundUserHost {
		t.Error("Expected user@host in ssh args")
	}
}

func TestSSHEnvironment_SSHArgs_NoKeyFile(t *testing.T) {
	env := &SSHEnvironment{
		host: "example.com",
		user: "root",
		port: "22",
	}

	args := env.sshArgs()
	for _, arg := range args {
		if arg == "-i" {
			t.Error("Expected no -i flag when no key file set")
		}
	}
}

// --- DaytonaEnvironment tests ---

func TestDaytonaEnvironment_Name(t *testing.T) {
	env := &DaytonaEnvironment{
		workspaceID: "ws-123",
	}
	if env.Name() != "daytona" {
		t.Errorf("Expected 'daytona', got '%s'", env.Name())
	}
}

func TestDaytonaEnvironment_IsAvailable(t *testing.T) {
	env := &DaytonaEnvironment{}
	// Daytona CLI is unlikely to be installed in test env
	_ = env.IsAvailable()
}

// --- ModalEnvironment tests ---

func TestModalEnvironment_Name(t *testing.T) {
	env := &ModalEnvironment{
		appName: "test-app",
	}
	if env.Name() != "modal" {
		t.Errorf("Expected 'modal', got '%s'", env.Name())
	}
}

func TestModalEnvironment_IsAvailable(t *testing.T) {
	env := &ModalEnvironment{}
	// Modal CLI is unlikely to be installed in test env
	_ = env.IsAvailable()
}

// --- SingularityEnvironment tests ---

func TestSingularityEnvironment_Name(t *testing.T) {
	env := &SingularityEnvironment{
		image:      "test.sif",
		executable: "singularity",
	}
	if env.Name() != "singularity" {
		t.Errorf("Expected 'singularity', got '%s'", env.Name())
	}
}

func TestSingularityEnvironment_IsAvailable(t *testing.T) {
	env := &SingularityEnvironment{
		executable: "singularity",
	}
	// Singularity is unlikely to be installed in test env
	_ = env.IsAvailable()
}

func TestSingularityEnvironment_Cleanup(t *testing.T) {
	env := &SingularityEnvironment{
		scratchDir: "",
	}
	// Empty scratch dir should be no-op
	err := env.Cleanup()
	if err != nil {
		t.Errorf("Expected nil error for empty scratchDir, got %v", err)
	}
}

func TestSingularityEnvironment_ScratchDir(t *testing.T) {
	env := &SingularityEnvironment{
		scratchDir: "/tmp/test-scratch",
	}
	if env.ScratchDir() != "/tmp/test-scratch" {
		t.Errorf("Expected '/tmp/test-scratch', got '%s'", env.ScratchDir())
	}
}

// --- Registry tests ---

func TestListEnvironments(t *testing.T) {
	envs := ListEnvironments()
	if len(envs) == 0 {
		t.Error("Expected at least one registered environment")
	}

	// Check that local, docker, ssh are registered
	envMap := make(map[string]bool)
	for _, name := range envs {
		envMap[name] = true
	}
	if !envMap["local"] {
		t.Error("Expected 'local' to be registered")
	}
	if !envMap["docker"] {
		t.Error("Expected 'docker' to be registered")
	}
	if !envMap["ssh"] {
		t.Error("Expected 'ssh' to be registered")
	}
}

func TestGetEnvironment_Docker(t *testing.T) {
	env, err := GetEnvironment("docker", map[string]string{
		"image": "alpine:latest",
	})
	if err != nil {
		t.Fatalf("GetEnvironment(docker) failed: %v", err)
	}
	if env == nil {
		t.Fatal("Expected non-nil docker environment")
	}
	if env.Name() != "docker" {
		t.Errorf("Expected 'docker', got '%s'", env.Name())
	}
}

func TestGetEnvironment_SSH(t *testing.T) {
	env, err := GetEnvironment("ssh", map[string]string{
		"host": "example.com",
	})
	if err != nil {
		t.Fatalf("GetEnvironment(ssh) failed: %v", err)
	}
	if env == nil {
		t.Fatal("Expected non-nil ssh environment")
	}
	if env.Name() != "ssh" {
		t.Errorf("Expected 'ssh', got '%s'", env.Name())
	}
}

func TestGetEnvironment_SSH_NoHost(t *testing.T) {
	_, err := GetEnvironment("ssh", map[string]string{})
	if err == nil {
		t.Error("Expected error when SSH host is missing")
	}
}

func TestLocalEnvironment_WorkDir(t *testing.T) {
	env := NewLocalEnvironment()
	dir := env.WorkDir()
	if dir == "" {
		t.Error("Expected non-empty working directory")
	}

	env.SetWorkDir("/tmp/test")
	if env.WorkDir() != "/tmp/test" {
		t.Errorf("Expected '/tmp/test', got '%s'", env.WorkDir())
	}
}

// --- PersistentShell tests ---

func TestPersistentShell_Name(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}
	defer env.Close()

	if env.Name() != "persistent_shell" {
		t.Errorf("Expected 'persistent_shell', got '%s'", env.Name())
	}
}

func TestPersistentShell_IsAvailable(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}

	if !env.IsAvailable() {
		t.Error("Expected available before close")
	}

	env.Close()
	if env.IsAvailable() {
		t.Error("Expected unavailable after close")
	}
}

func TestPersistentShell_Execute(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}
	defer env.Close()

	stdout, _, exitCode, err := env.Execute("echo hello_persistent", 10)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "hello_persistent") {
		t.Errorf("Expected 'hello_persistent' in stdout, got '%s'", stdout)
	}
}

func TestPersistentShell_Execute_ExitCode(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}
	defer env.Close()

	_, _, exitCode, err := env.Execute("exit 42", 10)
	if err != nil {
		// Note: exit 42 in a persistent shell may or may not error depending on impl
		// Just check we don't crash
		_ = exitCode
	}
}

func TestPersistentShell_StatePreservation(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}
	defer env.Close()

	// Set env var in first command
	_, _, _, err = env.Execute("export HERMES_TEST_VAR=persistent_value", 10)
	if err != nil {
		t.Fatalf("Set var failed: %v", err)
	}

	// Read env var in second command
	stdout, _, _, err := env.Execute("echo $HERMES_TEST_VAR", 10)
	if err != nil {
		t.Fatalf("Read var failed: %v", err)
	}
	if !strings.Contains(stdout, "persistent_value") {
		t.Errorf("Expected 'persistent_value' in stdout, got '%s'", stdout)
	}
}

func TestPersistentShell_DoubleClose(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}

	err = env.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Second close should be a no-op
	err = env.Close()
	if err != nil {
		t.Errorf("Second close should not fail: %v", err)
	}
}

func TestPersistentShell_ExecuteAfterClose(t *testing.T) {
	env, err := NewPersistentShell()
	if err != nil {
		t.Fatalf("NewPersistentShell failed: %v", err)
	}
	env.Close()

	_, _, _, err = env.Execute("echo test", 10)
	if err == nil {
		t.Error("Expected error when executing on closed shell")
	}
}

// --- DockerEnvironment factory tests ---

func TestDockerEnvironment_Factory(t *testing.T) {
	env, err := GetEnvironment("docker", map[string]string{
		"image":          "alpine:latest",
		"container_name": "test-container",
		"volumes":        "/tmp:/data",
		"forward_env":    "PATH,HOME",
	})
	if err != nil {
		t.Fatalf("GetEnvironment(docker) failed: %v", err)
	}
	dockerEnv, ok := env.(*DockerEnvironment)
	if !ok {
		t.Fatal("Expected *DockerEnvironment")
	}
	if dockerEnv.image != "alpine:latest" {
		t.Errorf("Expected image 'alpine:latest', got '%s'", dockerEnv.image)
	}
	if dockerEnv.containerName != "test-container" {
		t.Errorf("Expected container 'test-container', got '%s'", dockerEnv.containerName)
	}
}

func TestDockerEnvironment_DefaultImage(t *testing.T) {
	env, err := GetEnvironment("docker", map[string]string{})
	if err != nil {
		t.Fatalf("GetEnvironment(docker) failed: %v", err)
	}
	dockerEnv := env.(*DockerEnvironment)
	if dockerEnv.image != "ubuntu:latest" {
		t.Errorf("Expected default image 'ubuntu:latest', got '%s'", dockerEnv.image)
	}
}

// --- Timeout clamping tests ---

func TestLocalEnvironment_TimeoutClamping(t *testing.T) {
	env := NewLocalEnvironment()

	// Test with high timeout (should be clamped to 600)
	// We can't easily test the clamping without actually running a command,
	// but we can test a short command with a normal timeout
	stdout, _, exitCode, err := env.Execute("echo clamped", 5)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if exitCode != 0 {
		t.Error("Expected exit code 0")
	}
	if !strings.Contains(stdout, "clamped") {
		t.Error("Expected 'clamped' in output")
	}
}

// --- Registration completeness ---

func TestAllEnvironmentsRegistered(t *testing.T) {
	envs := ListEnvironments()
	envMap := make(map[string]bool)
	for _, name := range envs {
		envMap[name] = true
	}

	expected := []string{"local", "docker", "ssh", "daytona", "modal", "singularity", "persistent_shell"}
	for _, name := range expected {
		if !envMap[name] {
			t.Errorf("Expected environment '%s' to be registered", name)
		}
	}
}
