package environments

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// K8sJobEnvironment executes commands as Kubernetes Jobs.
// It uses kubectl for API interaction to avoid a heavy client-go dependency,
// making it compatible with any cluster that kubectl can reach.
type K8sJobEnvironment struct {
	namespace      string
	image          string
	cpuLimit       string
	memoryLimit    string
	serviceAccount string
}

func init() {
	RegisterEnvironment("k8s-job", func(params map[string]string) (Environment, error) {
		namespace := params["namespace"]
		if namespace == "" {
			namespace = os.Getenv("K8S_JOB_NAMESPACE")
		}
		if namespace == "" {
			namespace = "default"
		}

		image := params["image"]
		if image == "" {
			image = os.Getenv("K8S_JOB_IMAGE")
		}
		if image == "" {
			image = "ubuntu:latest"
		}

		cpuLimit := params["cpu_limit"]
		if cpuLimit == "" {
			cpuLimit = os.Getenv("K8S_JOB_CPU_LIMIT")
		}
		if cpuLimit == "" {
			cpuLimit = "500m"
		}

		memoryLimit := params["memory_limit"]
		if memoryLimit == "" {
			memoryLimit = os.Getenv("K8S_JOB_MEMORY_LIMIT")
		}
		if memoryLimit == "" {
			memoryLimit = "256Mi"
		}

		serviceAccount := params["service_account"]
		if serviceAccount == "" {
			serviceAccount = os.Getenv("K8S_JOB_SERVICE_ACCOUNT")
		}

		return &K8sJobEnvironment{
			namespace:      namespace,
			image:          image,
			cpuLimit:       cpuLimit,
			memoryLimit:    memoryLimit,
			serviceAccount: serviceAccount,
		}, nil
	})
}

// Execute runs a command as a Kubernetes Job and waits for completion.
func (e *K8sJobEnvironment) Execute(command string, timeout int) (stdout, stderr string, exitCode int, err error) {
	if timeout <= 0 {
		timeout = 120
	}
	if timeout > 600 {
		timeout = 600
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	jobName := fmt.Sprintf("hermesx-exec-%s", uuid.New().String()[:8])

	// Create the Job manifest and apply it.
	manifest := e.buildJobManifest(jobName, command, timeout)
	if err := e.applyManifest(ctx, manifest); err != nil {
		return "", "", -1, fmt.Errorf("k8s-job: failed to create job: %w", err)
	}

	// Ensure cleanup regardless of outcome.
	defer e.deleteJob(jobName)

	// Wait for the Job to complete.
	if err := e.waitForCompletion(ctx, jobName, timeout); err != nil {
		return "", "", -1, err
	}

	// Retrieve logs from the Job's pod.
	stdout, stderr, exitCode, err = e.getLogs(ctx, jobName)
	if err != nil {
		return stdout, stderr, -1, fmt.Errorf("k8s-job: failed to retrieve logs: %w", err)
	}

	return stdout, stderr, exitCode, nil
}

// IsAvailable checks if kubectl is installed and can reach the cluster.
func (e *K8sJobEnvironment) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Name returns "k8s-job".
func (e *K8sJobEnvironment) Name() string {
	return "k8s-job"
}

// buildJobManifest generates a Kubernetes Job YAML manifest.
func (e *K8sJobEnvironment) buildJobManifest(jobName, command string, timeout int) string {
	// Escape command for embedding in YAML.
	escapedCmd := strings.ReplaceAll(command, "'", "'\"'\"'")

	saBlock := ""
	if e.serviceAccount != "" {
		saBlock = fmt.Sprintf("      serviceAccountName: %s\n", e.serviceAccount)
	}

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/managed-by: hermesx
    hermesx.io/component: sandbox-exec
spec:
  backoffLimit: 0
  activeDeadlineSeconds: %d
  ttlSecondsAfterFinished: 60
  template:
    metadata:
      labels:
        app.kubernetes.io/managed-by: hermesx
        hermesx.io/job-name: %s
    spec:
      restartPolicy: Never
%s      containers:
      - name: exec
        image: %s
        command: ["sh", "-c", '%s']
        resources:
          limits:
            cpu: "%s"
            memory: "%s"
          requests:
            cpu: "100m"
            memory: "64Mi"
`,
		jobName, e.namespace,
		timeout,
		jobName,
		saBlock,
		e.image,
		escapedCmd,
		e.cpuLimit, e.memoryLimit,
	)
}

// applyManifest applies a YAML manifest using kubectl.
func (e *K8sJobEnvironment) applyManifest(ctx context.Context, manifest string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderrBuf.String())
	}
	return nil
}

// waitForCompletion polls the Job status until it completes or the context expires.
func (e *K8sJobEnvironment) waitForCompletion(ctx context.Context, jobName string, timeout int) error {
	// Use kubectl wait which blocks until the condition is met.
	cmd := exec.CommandContext(ctx, "kubectl", "wait",
		"--for=condition=complete",
		"--timeout", fmt.Sprintf("%ds", timeout),
		"-n", e.namespace,
		fmt.Sprintf("job/%s", jobName),
	)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err == nil {
		return nil
	}

	// Check if the job failed (as opposed to timed out).
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("k8s-job: command timed out after %d seconds", timeout)
	}

	// kubectl wait returns an error for failed jobs too; check if the job
	// actually failed vs just not being complete yet.
	failCmd := exec.CommandContext(ctx, "kubectl", "wait",
		"--for=condition=failed",
		"--timeout", "5s",
		"-n", e.namespace,
		fmt.Sprintf("job/%s", jobName),
	)
	if failCmd.Run() == nil {
		// Job failed — we'll still get logs, just report a non-zero exit code.
		return nil
	}

	return fmt.Errorf("k8s-job: wait failed: %w: %s", err, stderrBuf.String())
}

// getLogs retrieves stdout/stderr from the Job's pod.
func (e *K8sJobEnvironment) getLogs(ctx context.Context, jobName string) (stdout, stderr string, exitCode int, err error) {
	// Get logs from the pod created by the job.
	cmd := exec.CommandContext(ctx, "kubectl", "logs",
		"-n", e.namespace,
		fmt.Sprintf("job/%s", jobName),
	)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}

	// Determine exit code from the Job status.
	exitCode = e.getJobExitCode(ctx, jobName)

	return stdout, stderr, exitCode, nil
}

// getJobExitCode queries the pod's container exit code.
func (e *K8sJobEnvironment) getJobExitCode(ctx context.Context, jobName string) int {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", e.namespace,
		"-l", fmt.Sprintf("hermesx.io/job-name=%s", jobName),
		"-o", "jsonpath={.items[0].status.containerStatuses[0].state.terminated.exitCode}",
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return -1
	}

	result := strings.TrimSpace(out.String())
	if result == "" {
		return -1
	}

	var code int
	if _, err := fmt.Sscanf(result, "%d", &code); err != nil {
		return -1
	}
	return code
}

// deleteJob removes the Job and its pods.
func (e *K8sJobEnvironment) deleteJob(jobName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "delete", "job",
		jobName,
		"-n", e.namespace,
		"--ignore-not-found=true",
		"--cascade=foreground",
	)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		slog.Warn("k8s-job: failed to delete job",
			"job", jobName,
			"namespace", e.namespace,
			"error", err,
			"stderr", stderrBuf.String(),
		)
	}
}
