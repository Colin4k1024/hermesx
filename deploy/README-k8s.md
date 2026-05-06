# Hermes Agent — Kubernetes Deployment Guide

Comprehensive guide for deploying the Hermes Agent SaaS platform on Kubernetes,
covering Helm (recommended), Kustomize (alternative), monitoring, ingress, autoscaling, and common troubleshooting.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Quick Start — Helm (Recommended)](#3-quick-start--helm--recommended)
4. [Quick Start — Kustomize (Alternative)](#4-quick-start--kustomize--alternative)
5. [Service Access Reference](#5-service-access-reference)
6. [Health Probe Reference](#6-health-probe-reference)
7. [Prometheus & Grafana Setup](#7-prometheus--grafana-setup)
8. [Ingress Setup](#8-ingress-setup)
9. [Running the Bootstrap Job](#9-running-the-bootstrap-job)
10. [HPA — Horizontal Pod Autoscaling](#10-hpa--horizontal-pod-autoscaling)
11. [Customization](#11-customization)
12. [Troubleshooting](#12-troubleshooting)
13. [Cleanup](#13-cleanup)
14. [Prometheus Metrics Reference](#14-prometheus-metrics-reference)

---

## 1. Overview

Hermes Agent is a multi-tenant AI agent platform that can operate in SaaS mode on Kubernetes. This guide covers:

- **Hermes Agent** (`hermes-agent-saas` binary): Core SaaS API server exposing REST endpoints, health probes, and Prometheus metrics.
- **Hermes WebUI**: Multi-tenant React frontend for agent chat.
- **PostgreSQL 16**: Per-tenant data store (user memory, conversation history, skills registry).
- **MinIO**: Object storage for per-tenant soul files and skill bundles.
- **Bootstrap Job**: Idempotent job that seeds two test tenants (IsolationTest-Pirate, IsolationTest-Academic) and writes `tests/fixtures/tenants.json`.
- **Prometheus + Grafana**: Optional monitoring stack.
- **Ingress**: Optional NGINX ingress for cluster-wide access.

---

## 2. Prerequisites

### Core Tools

| Tool | Minimum Version | Required For |
|---|---|---|
| `kubectl` | 1.27+ | All K8s operations |
| `Helm` | 3.12+ | Helm deployment (recommended) |
| `docker` | Latest | Building and loading local images |

### Local Cluster

| Tool | Required For | Notes |
|---|---|---|
| `kind` | Local Kind cluster | Recommended for local dev |
| `minikube` | Local Minikube cluster | Alternative to Kind |

> **Note:** Ensure your cluster has at least 4 CPU cores and 8 GB RAM available.

### Optional

| Tool | Required For |
|---|---|
| `kustomize` | Kustomize overlay deployment (alternative to Helm) |
| `helm-docs` | Auto-generating values documentation |
| `stern` | Streaming pod logs |
| `jq` | JSON query in troubleshooting |

---

## 3. Quick Start — Helm (Recommended)

This is the recommended deployment path. It uses the Helm chart at `deploy/helm/hermes-agent/` and the `values.local.yaml` overlay for local Kind/Minikube clusters.

### Step 1 — Create the namespace

```bash
kubectl create namespace hermes
```

### Step 2 — Build the hermes-agent Docker image

From the repository root:

```bash
docker build -f Dockerfile.saas -t hermes-agent-saas:local .
```

### Step 3 — Build the hermes-webui Docker image

```bash
docker build -f webui/Dockerfile -t hermes-webui:local .
```

### Step 4 — Load images into Kind (if using Kind)

```bash
kind load docker-image hermes-agent-saas:local --name kind-hermes
kind load docker-image hermes-webui:local --name kind-hermes
```

Replace `--name kind-hermes` with your actual Kind cluster name if different.

### Step 5 — Install with Helm

```bash
cd deploy/helm/hermes-agent
helm dependency build
helm install hermes ./hermes-agent \
  -f ./values.local.yaml \
  --namespace hermes \
  --create-namespace \
  --wait --timeout 5m
```

> **Note:** `values.local.yaml` is pre-configured for a Kind cluster with PostgreSQL (`postgres-postgresql` headless service) and MinIO running locally. It sets `postgresql.enabled: false` and `service.type: NodePort`.

### Step 6 — Check pod status

```bash
kubectl get pods -n hermes
```

All pods should reach `Running` with all containers ready. Allow ~30 seconds for MinIO to initialize on first deploy.

Expected output:

```
NAME                            READY   STATUS    RESTARTS   AGE
hermes-agent-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
hermes-postgresql-0             1/1     Running   0          30s
hermes-webui-xxxxxxxxxx-xxxxx   1/1     Running   0          20s
minio-0                         1/1     Running   0          30s
```

### Step 7 — Port-forward for local access

```bash
# Hermes Agent SaaS API
kubectl port-forward svc/hermes-agent 8080:8080 -n hermes &

# Hermes WebUI
kubectl port-forward svc/hermes-webui 3000:80 -n hermes &
```

### Step 8 — Verify the deployment

```bash
# Liveness probe — confirms the process is alive
curl http://localhost:8080/health/live
# → {"status":"alive"}

# Readiness probe — confirms all dependencies (DB, Redis, MinIO) are healthy
curl http://localhost:8080/health/ready

# Prometheus metrics
curl http://localhost:8080/metrics | head -20
```

---

## 4. Quick Start — Kustomize (Alternative)

Use this path if you prefer Kustomize overlays over Helm templating, or if you are deploying the flat K8s manifests directly.

### Step 1 — Copy and edit the environment config

```bash
cp deploy/k8s/quickstart/config.env.example deploy/k8s/quickstart/config.env
```

Edit `config.env` and fill in your values:

```env
# LLM base URL — MUST include /v1 suffix
LLM_API_URL=https://api.minimaxi.com/v1
LLM_API_KEY=sk-your-api-key-here
LLM_MODEL=MiniMax-M2.7-highspeed
HERMES_ACP_TOKEN=dev-bootstrap-token
```

### Step 2 — Apply all resources

```bash
kubectl apply -k deploy/k8s/quickstart/
```

This deploys:
- `hermes-bootstrap` Job
- `hermes-webui` Deployment + Service
- `hermes-llm-config` Secret (from `config.env`)
- `hermes-bootstrap-script` ConfigMap

> **Prerequisite:** `hermes-agent` Deployment must already exist in the cluster. If using the Helm path (Section 3), run `helm install` first before applying Kustomize overlays.

### Step 3 — Check status

```bash
kubectl get all -n hermes
kubectl get pods -n hermes --watch
```

### Step 4 — Access services

| Service | Local URL |
|---|---|
| hermes-agent | http://localhost:30080 |
| hermes-webui | http://localhost:30081 |
| minio-api | http://localhost:30090 |
| minio-console | http://localhost:30091 |

Port-forward if needed:

```bash
kubectl port-forward svc/hermes-agent 30080:8080 -n hermes &
kubectl port-forward svc/hermes-webui 30081:80 -n hermes &
kubectl port-forward svc/minio-api 30090:9000 -n hermes &
kubectl port-forward svc/minio-console 30091:9001 -n hermes &
```

---

## 5. Service Access Reference

| Service | Type | Internal Port | NodePort | Local Access URL | Path |
|---|---|---|---|---|---|
| hermes-agent | NodePort | 8080 | 30080 | http://localhost:30080 | / |
| hermes-webui | NodePort | 80 | 30081 | http://localhost:30081 | / |
| minio-api | NodePort | 9000 | 30090 | http://localhost:30090 | / |
| minio-console | NodePort | 9001 | 30091 | http://localhost:30091 | / |
| postgres | Headless | 5432 | N/A | Via ClusterIP only | N/A |
| minio | Headless | 9000/9001 | N/A | Via ClusterIP only | N/A |

### MinIO Console Credentials

| Field | Default Value |
|---|---|
| Access Key | `hermes-minio` |
| Secret Key | `hermes-minio-password` |

---

## 6. Health Probe Reference

| Endpoint | Method | K8s Probe | Purpose | Success Response |
|---|---|---|---|---|
| `/health/live` | GET | Liveness | Confirms the process is alive | `{"status":"alive"}` |
| `/health/ready` | GET | Readiness | Confirms all dependencies (DB, Redis, MinIO) are reachable | Full health JSON with component status |
| `/metrics` | GET | — | Prometheus metrics scrape target | Prometheus text exposition format |

### Liveness vs Readiness

- **Liveness** (`/health/live`): Use for `livenessProbe`. If this fails, K8s restarts the container. Should be cheap (process alive) with no dependency checks.
- **Readiness** (`/health/ready`): Use for `readinessProbe`. If this fails, K8s removes the pod from Service endpoints. Performs full dependency checks (PostgreSQL, Redis, MinIO).

### MinIO Health

```bash
curl http://localhost:30090/minio/health/live
curl http://localhost:30090/minio/health/ready
```

---

## 7. Prometheus & Grafana Setup

### Prerequisites

The Helm chart ships a Prometheus `ServiceMonitor` (for `prometheus-operator`) and a sidecar scrape annotation on the pod:

```yaml
# Pod annotations (already in deployment.yaml)
prometheus.io/scrape: "true"
prometheus.io/port: "8080"
prometheus.io/path: "/metrics"
```

### Deploy Prometheus Operator + Grafana via Helm

```bash
# Add kube-prometheus-stack if not already present
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install kube-prometheus-stack
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.adminPassword=prometheus \
  --wait

# Verify
kubectl get pods -n monitoring
```

### Port-forward to access dashboards

```bash
# Prometheus UI
kubectl port-forward svc/prometheus-kube-prometheus-stack-prometheus 9090:9090 -n monitoring &

# Grafana (default credentials: admin / prometheus)
kubectl port-forward svc/prometheus-grafana 3000:3000 -n monitoring &
```

### Access

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin / prometheus)

### Useful PromQL queries

```promql
# HTTP request rate by status code
rate(hermes_http_requests_total[5m])

# HTTP request latency p99
histogram_quantile(0.99, rate(hermes_http_request_duration_seconds_bucket[5m]))

# PostgreSQL query latency p95
histogram_quantile(0.95, rate(hermes_pg_query_duration_seconds_bucket[5m]))
```

---

## 8. Ingress Setup

### Step 1 — Install the NGINX Ingress Controller (Kind)

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Wait for the controller to be ready
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s
```

### Step 2 — Enable Ingress via Helm upgrade

```bash
cd deploy/helm/hermes-agent
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  --set ingress.enabled=true \
  --set ingress.host=hermes.local \
  -n hermes \
  --wait
```

Or, enable via your custom values file:

```yaml
# values-ingress.yaml
ingress:
  enabled: true
  host: hermes.local
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/cors-allow-origin: "*"
```

```bash
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  -f ./values-ingress.yaml \
  -n hermes \
  --wait
```

### Step 3 — Add to /etc/hosts

```bash
echo "127.0.0.1 hermes.local" | sudo tee -a /etc/hosts
```

### Step 4 — Access via Ingress

```bash
# Agent API
curl -H "Host: hermes.local" http://localhost/api/health/live

# WebUI
curl -H "Host: hermes.local" http://localhost/
```

---

## 9. Running the Bootstrap Job

The bootstrap Job (`hermes-bootstrap`) is idempotent — running it multiple times is safe. It creates:

- Tenant: `IsolationTest-Pirate` (Captain Hermes persona, Treasure Hunt skill)
- Tenant: `IsolationTest-Academic` (Professor Hermes persona, Academic Research skill)
- Per-tenant API keys
- Per-tenant soul files in MinIO (`SOUL.md`)
- Test fixtures file: `tests/fixtures/tenants.json`

### Option A — Helm (recommended)

```bash
cd deploy/helm/hermes-agent
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  --set bootstrap.enabled=true \
  -n hermes \
  --wait
```

The Helm chart must be installed first (Section 3). The bootstrap Job reads `HERMES_ACP_TOKEN` from the `hermes-llm-config` Secret created by the chart.

### Option B — Standalone Kustomize

```bash
# Apply quickstart overlay (includes bootstrap Job)
kubectl apply -k deploy/k8s/quickstart/

# Wait for completion
kubectl wait --for=condition=complete job/hermes-bootstrap --timeout=120s -n hermes
```

### Verify bootstrap success

```bash
# Check job status
kubectl get job hermes-bootstrap -n hermes

# View logs
kubectl logs job/hermes-bootstrap -n hermes

# Read test fixtures
kubectl exec -n hermes deploy/hermes-agent -- cat /var/lib/hermes/tests/fixtures/tenants.json
```

Expected log output:

```
🚀 Hermes Quickstart Bootstrap
   BASE_URL:        http://hermes-agent:8080
   MINIO_ENDPOINT:  minio:9000
   FIXTURES_DIR:    /tmp/fixtures
⏳ Waiting for hermes-saas at http://hermes-agent:8080 ...
✅ hermes-saas is ready
⬇️  Downloading MinIO mc client...
✅ mc installed
🏴‍☠️  Creating IsolationTest-Pirate tenant...
   ID: <uuid>
🎓 Creating IsolationTest-Academic tenant...
   ID: <uuid>
🔑 Creating API keys...
   Pirate:    <key>...
   Academic:  <key>...
👻 Uploading souls to MinIO...
   ✅ Soul uploaded → <pirate-id>/SOUL.md
   ✅ Soul uploaded → <academic-id>/SOUL.md
🛠  Uploading exclusive skills...
   ✅ Skill uploaded: treasure-hunt
   ✅ Skill uploaded: academic-research
💾 Writing test fixtures...
   ✅ Written: /tmp/fixtures/tenants.json
✅ Bootstrap complete!
```

### Run bootstrap again (idempotent)

```bash
kubectl delete job hermes-bootstrap -n hermes
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  --set bootstrap.enabled=true \
  -n hermes \
  --wait
```

---

## 10. HPA — Horizontal Pod Autoscaling

### Enable autoscaling via Helm

```bash
cd deploy/helm/hermes-agent
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=1 \
  --set autoscaling.maxReplicas=5 \
  --set autoscaling.targetCPUUtilizationPercentage=70 \
  -n hermes \
  --wait
```

Or via values file:

```yaml
# values-hpa.yaml
autoscaling:
  enabled: true
  minReplicas: 1
  maxReplicas: 5
  targetCPUUtilizationPercentage: 70
```

```bash
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  -f ./values-hpa.yaml \
  -n hermes \
  --wait
```

### Verify HPA status

```bash
kubectl get hpa -n hermes
kubectl describe hpa hermes-agent -n hermes
```

### Load testing

```bash
# Install hey (lightweight HTTP load tester)
go install github.com/rakyll/hey@latest

# Generate load
hey -n 1000 -c 20 http://localhost:8080/health/ready

# Observe HPA scaling
kubectl get hpa hermes-agent -n hermes --watch
```

### HPA with custom metrics (optional)

For memory-based autoscaling, deploy the `metrics-server` component:

```bash
kubectl apply -f deploy/k8s/metrics-server/
kubectl get apiservice v1beta1.metrics.k8s.io -o jsonpath='{.status}'
```

Then reference `memory` in the HPA spec:

```yaml
metrics:
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## 11. Customization

### Override values with a custom values file

Always start from `values.local.yaml` and layer your overrides:

```bash
helm upgrade hermes ./hermes-agent \
  -f ./values.local.yaml \
  -f ./values-custom.yaml \
  -n hermes \
  --wait
```

### Use an external PostgreSQL database

```yaml
# values-external-pg.yaml
postgresql:
  enabled: false

env:
  DATABASE_URL: "postgres://hermes:your-password@pg.example.com:5432/hermes?sslmode=require"
```

### Use an external Redis instance

```yaml
env:
  REDIS_URL: "redis://redis.example.com:6379/0"
```

### Configure a custom LLM provider

```yaml
env:
  LLM_API_URL: "https://api.openai.com/v1"
  LLM_API_KEY: "sk-your-key"
  LLM_MODEL: "gpt-4o"
```

### Change the service type

```yaml
# ClusterIP (internal only)
service:
  type: ClusterIP

# LoadBalancer (cloud)
service:
  type: LoadBalancer
```

### Disable MinIO (use external object storage)

```yaml
minio:
  enabled: false

env:
  MINIO_ENDPOINT: "s3.amazonaws.com"
  MINIO_ACCESS_KEY: "your-access-key"
  MINIO_SECRET_KEY: "your-secret-key"
  MINIO_BUCKET: "hermes-skills"
```

### Resource limits

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi
```

---

## 12. Troubleshooting

### Pods not starting

**Symptom:** Pods are in `Pending` or `CrashLoopBackOff` state.

**Diagnosis:**

```bash
# Check pod events and status
kubectl describe pod <pod-name> -n hermes

# Check container logs
kubectl logs <pod-name> -n hermes --previous

# Check if images exist in the cluster
kubectl get nodes -o wide
docker images | grep hermes
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| Image not loaded in Kind | `kind load docker-image hermes-agent-saas:local --name kind-hermes` |
| `imagePullPolicy` is not `Never` for local images | Ensure `image.pullPolicy: Never` in values |
| Out-of-memory kill | Increase `resources.limits.memory` |
| Missing secret or configmap | Check `kubectl get secret -n hermes` and `kubectl get cm -n hermes` |

---

### Health probe failures

**Symptom:** Pods start but fail liveness or readiness probes, causing restarts or removal from endpoints.

**Diagnosis:**

```bash
# Port-forward and test manually
kubectl port-forward svc/hermes-agent 8080:8080 -n hermes
curl -v http://localhost:8080/health/live
curl -v http://localhost:8080/health/ready

# Check pod probe configuration
kubectl get pod <pod-name> -n hermes -o jsonpath='{.spec.containers[*].livenessProbe}'
kubectl get pod <pod-name> -n hermes -o jsonpath='{.spec.containers[*].readinessProbe}'
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| Database not ready | Ensure PostgreSQL pod is `Running` before hermes-agent starts. Add `initContainers` or increase `initialDelaySeconds` |
| `DATABASE_URL` incorrect | Verify `env.DATABASE_URL` in values matches the PostgreSQL service address |
| Readiness probe too aggressive | Increase `readiness.initialDelaySeconds` (default: 10s) or `periodSeconds` |

---

### MinIO not ready

**Symptom:** `/health/ready` returns MinIO as unhealthy, or bootstrap Job fails with `mc: Unable to initialize new alias`.

**Diagnosis:**

```bash
# Check MinIO pod
kubectl get pod minio-0 -n hermes

# Test MinIO health endpoint
kubectl port-forward svc/minio-api 9000:9000 -n hermes &
curl http://localhost:9000/minio/health/live
```

**Fix:**

- Wait 30 seconds after first deploy — MinIO takes time to initialize its data directory on first start.
- Verify `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, and `MINIO_SECRET_KEY` env vars match the values in `deploy/kind/minio.yaml`.

---

### Bootstrap Job fails

**Symptom:** `hermes-bootstrap` Job stays in `Running` or `Failed` state.

**Diagnosis:**

```bash
kubectl logs job/hermes-bootstrap -n hermes
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| `hermes-agent` not ready | Ensure hermes-agent is `Running` and `/health/ready` returns 200 before applying bootstrap |
| Wrong `HERMES_ACP_TOKEN` | Ensure `HERMES_ACP_TOKEN` in `config.env` / Secret matches `env.HERMES_ACP_TOKEN` in values |
| MinIO not initialized | Wait 30s, then re-run the Job |
| Network policy blocking | Ensure the Job pod can reach `hermes-agent:8080` and `minio:9000` |
| Image pull failure | Ensure `alpine:3.19` is accessible or pull it manually: `docker pull alpine:3.19` |

**Re-run the Job:**

```bash
kubectl delete job hermes-bootstrap -n hermes
# Then re-apply via helm upgrade with --set bootstrap.enabled=true
# or via kubectl apply -k deploy/k8s/quickstart/
```

---

### ImagePullBackOff

**Symptom:** Pod stuck in `ImagePullBackOff`.

**Fix:**

```bash
# For Kind — ensure the image is loaded
kind load docker-image hermes-agent-saas:local --name kind-hermes

# For Minikube — use the internal registry
minikube image load hermes-agent-saas:local

# Verify image is available in the cluster node
kubectl run debug --image=hermes-agent-saas:local --rm -it --restart=Never -- echo "image exists"
```

---

### HPA not scaling

**Diagnosis:**

```bash
kubectl describe hpa hermes-agent -n hermes
kubectl top pods -n hermes
kubectl describe deployment hermes-agent -n hermes
```

**Common causes:**

- `metrics-server` not installed — install with `kubectl apply -f deploy/k8s/metrics-server/`
- CPU/memory not under pressure — load test to trigger scaling
- HPA max replicas reached — increase `autoscaling.maxReplicas`

---

## 13. Cleanup

### Helm uninstall

```bash
helm uninstall hermes -n hermes
```

### Delete namespaces

```bash
kubectl delete namespace hermes
kubectl delete namespace monitoring
```

### Delete Kind cluster

```bash
kind delete cluster --name kind-hermes
```

### Delete Minikube cluster

```bash
minikube delete
```

### Remove /etc/hosts entry (if added)

```bash
sudo sed -i '' '/hermes.local/d' /etc/hosts
```

---

## 14. Prometheus Metrics Reference

The following metrics are exposed by `hermes-agent` at `GET /metrics`.

### Database Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `hermes_pg_query_duration_seconds` | Histogram | `operation` | PostgreSQL query latency via `database/sql` |
| `hermes_pgx_query_duration_seconds` | Histogram | `operation` | Direct pgx query latency |

### HTTP Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `hermes_http_requests_total` | Counter | `method`, `path`, `status_code` | Total HTTP request count |
| `hermes_http_request_duration_seconds` | Histogram | `method`, `path` | HTTP request latency distribution |

### Process Metrics (standard)

| Metric | Type | Description |
|---|---|---|
| `go_info` | Gauge | Go version info |
| `go_goroutines` | Gauge | Number of goroutines |
| `go_memstats_alloc_bytes` | Gauge | Memory allocated |
| `process_cpu_seconds_total` | Counter | CPU time consumed |
| `process_open_fds` | Gauge | Open file descriptors |
| `up` | Gauge | Target up (1) or down (0) |

### Example queries

```promql
# Request rate per second
rate(hermes_http_requests_total[5m])

# Error rate
rate(hermes_http_requests_total{status_code=~"5.."}[5m])

# p99 request latency
histogram_quantile(0.99, rate(hermes_http_request_duration_seconds_bucket[5m]))

# p95 PostgreSQL query latency
histogram_quantile(0.95, rate(hermes_pg_query_duration_seconds_bucket[5m]))
```

---

## Appendix A — Full Environment Variable Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string. Format: `postgres://user:pass@host:5432/dbname?sslmode=disable` |
| `HERMES_ACP_TOKEN` | Yes | `dev-token-...` | Static bearer token for ACP admin endpoints |
| `SAAS_API_PORT` | No | `8080` | Port for the SaaS API server |
| `SAAS_ALLOWED_ORIGINS` | No | `*` | CORS allowed origins (comma-separated or `*`) |
| `SAAS_STATIC_DIR` | No | `/static` | Directory for static file serving; empty = disabled |
| `HERMES_API_PORT` | No | `8081` | Port for the OpenAI-compatible adapter |
| `HERMES_API_KEY` | No | `dev-api-key-...` | API key for the OpenAI-compatible adapter |
| `REDIS_URL` | No | empty | Redis URL for session/caching. Empty = Redis disabled |
| `LLM_API_URL` | No | empty | Base URL for LLM backend (must include `/v1`) |
| `LLM_API_KEY` | No | empty | API key for LLM backend |
| `LLM_MODEL` | No | `Qwen3-Coder-Next-4bit` | Default LLM model name |
| `MINIO_ENDPOINT` | No | empty | MinIO API endpoint (`host:port`) |
| `MINIO_ACCESS_KEY` | No | empty | MinIO access key |
| `MINIO_SECRET_KEY` | No | empty | MinIO secret key |
| `MINIO_BUCKET` | No | empty | MinIO bucket name for soul/skill storage |

---

## Appendix B — File Structure

```
deploy/
├── README-k8s.md              ← This guide
├── helm/
│   └── hermes-agent/
│       ├── Chart.yaml
│       ├── values.yaml        ← Default values
│       └── templates/
│           ├── deployment.yaml
│           └── service.yaml
├── kind/
│   ├── values.local.yaml      ← Kind/Minikube overlay
│   ├── postgres.yaml          ← PostgreSQL StatefulSet
│   └── minio.yaml             ← MinIO StatefulSet + Secret
└── k8s/
    ├── quickstart/
    │   ├── kustomization.yaml
    │   ├── config.env.example  ← LLM credentials template
    │   ├── bootstrap.sh       ← Idempotent tenant seeder
    │   ├── bootstrap-job.yaml
    │   └── webui-deployment.yaml
    └── full/
        └── (all-in-one overlay for production-style deploy)
```
