# HermesX — Kubernetes SaaS Deployment Guide

Comprehensive guide for deploying the HermesX SaaS service on Kubernetes,
covering Helm (recommended), Kustomize bootstrap overlays, monitoring, ingress, autoscaling, and common troubleshooting.

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

HermesX is deployed as a SaaS-only, multi-tenant AI agent service on Kubernetes. This guide covers:

- **HermesX SaaS API** (`hermesx saas-api`): Core service exposing REST endpoints, embedded WebUI static files, health probes, and Prometheus metrics.
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

This is the recommended deployment path. It uses the Helm chart at `deploy/helm/hermesx/` and the `deploy/kind/values.local.yaml` overlay for local Kind/Minikube clusters.

### Step 1 — Create the namespace

```bash
kubectl create namespace hermesx
```

### Step 2 — Build the HermesX SaaS image

From the repository root:

```bash
docker build -f Dockerfile.saas -t hermesx/hermesx-saas:local .
```

### Step 3 — Load the image into Kind (if using Kind)

```bash
kind load docker-image hermesx/hermesx-saas:local --name kind-hermes
```

Replace `--name kind-hermes` with your actual Kind cluster name if different.

### Step 4 — Install with Helm

```bash
helm install hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  --namespace hermesx \
  --create-namespace \
  --wait --timeout 5m
```

> **Note:** `deploy/kind/values.local.yaml` is pre-configured for a Kind cluster with external MySQL, Redis, and S3-compatible object storage services. It sets `service.type: NodePort` and serves the WebUI from `SAAS_STATIC_DIR=/static` inside the SaaS image.

### Step 5 — Check pod status

```bash
kubectl get pods -n hermesx
```

All pods should reach `Running` with all containers ready. Allow ~30 seconds for MinIO to initialize on first deploy.

Expected output:

```
NAME                            READY   STATUS    RESTARTS   AGE
hermesx-xxxxxxxxxx-xxxxx        1/1     Running   0          30s
mysql-0                         1/1     Running   0          30s
redis-0                         1/1     Running   0          30s
rustfs-0                        1/1     Running   0          30s
```

### Step 6 — Port-forward for local access

```bash
# HermesX SaaS API and embedded WebUI
kubectl port-forward svc/hermesx 8080:8080 -n hermesx &
```

### Step 7 — Verify the deployment

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

Use this path after deploying the base SaaS service with Helm when you need to run the bootstrap Job and inject local LLM configuration through Kustomize.

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
- `hermesx-bootstrap` Job
- `hermes-llm-config` Secret (from `config.env`)
- `hermesx-bootstrap-script` ConfigMap

> **Prerequisite:** `hermesx` SaaS Deployment must already exist in the cluster. If using the Helm path (Section 3), run `helm install` first before applying Kustomize overlays.

### Step 3 — Check status

```bash
kubectl get all -n hermesx
kubectl get pods -n hermesx --watch
```

### Step 4 — Access services

| Service | Local URL |
|---|---|
| hermesx SaaS API and WebUI | http://localhost:30080 |
| minio-api | http://localhost:30090 |
| minio-console | http://localhost:30091 |

Port-forward if needed:

```bash
kubectl port-forward svc/hermesx 30080:8080 -n hermesx &
kubectl port-forward svc/minio-api 30090:9000 -n hermesx &
kubectl port-forward svc/minio-console 30091:9001 -n hermesx &
```

---

## 5. Service Access Reference

| Service | Type | Internal Port | NodePort | Local Access URL | Path |
|---|---|---|---|---|---|
| hermesx | NodePort | 8080 | 30080 | http://localhost:30080 | / |
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
helm upgrade hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  --set ingress.enabled=true \
  --set ingress.host=hermes.local \
  -n hermesx \
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
helm upgrade hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  -f ./values-ingress.yaml \
  -n hermesx \
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

The bootstrap Job (`hermesx-bootstrap`) is idempotent — running it multiple times is safe. It creates:

- Tenant: `IsolationTest-Pirate` (Captain Hermes persona, Treasure Hunt skill)
- Tenant: `IsolationTest-Academic` (Professor Hermes persona, Academic Research skill)
- Per-tenant API keys
- Per-tenant soul files in MinIO (`SOUL.md`)
- Test fixtures file: `tests/fixtures/tenants.json`

### Apply the Kustomize Overlay

```bash
# Apply quickstart overlay (includes bootstrap Job)
kubectl apply -k deploy/k8s/quickstart/

# Wait for completion
kubectl wait --for=condition=complete job/hermesx-bootstrap --timeout=120s -n hermesx
```

### Verify bootstrap success

```bash
# Check job status
kubectl get job hermesx-bootstrap -n hermesx

# View logs
kubectl logs job/hermesx-bootstrap -n hermesx

# Read test fixtures
kubectl exec -n hermesx deploy/hermesx -- cat /var/lib/hermes/tests/fixtures/tenants.json
```

Expected log output:

```
🚀 Hermes Quickstart Bootstrap
   BASE_URL:        http://hermesx:8080
   MINIO_ENDPOINT:  minio:9000
   FIXTURES_DIR:    /tmp/fixtures
⏳ Waiting for hermesx at http://hermesx:8080 ...
✅ hermesx is ready
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
kubectl delete job hermesx-bootstrap -n hermesx
kubectl apply -k deploy/k8s/quickstart/
```

---

## 10. HPA — Horizontal Pod Autoscaling

### Enable autoscaling via Helm

```bash
helm upgrade hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=1 \
  --set autoscaling.maxReplicas=5 \
  --set autoscaling.targetCPUUtilizationPercentage=70 \
  -n hermesx \
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
helm upgrade hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  -f ./values-hpa.yaml \
  -n hermesx \
  --wait
```

### Verify HPA status

```bash
kubectl get hpa -n hermesx
kubectl describe hpa hermesx -n hermesx
```

### Load testing

```bash
# Install hey (lightweight HTTP load tester)
go install github.com/rakyll/hey@latest

# Generate load
hey -n 1000 -c 20 http://localhost:8080/health/ready

# Observe HPA scaling
kubectl get hpa hermesx -n hermesx --watch
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
helm upgrade hermesx deploy/helm/hermesx \
  -f deploy/kind/values.local.yaml \
  -f ./values-custom.yaml \
  -n hermesx \
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
kubectl describe pod <pod-name> -n hermesx

# Check container logs
kubectl logs <pod-name> -n hermesx --previous

# Check if images exist in the cluster
kubectl get nodes -o wide
docker images | grep hermes
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| Image not loaded in Kind | `kind load docker-image hermesx/hermesx-saas:local --name kind-hermes` |
| `imagePullPolicy` is not `Never` for local images | Ensure `image.pullPolicy: Never` in values |
| Out-of-memory kill | Increase `resources.limits.memory` |
| Missing secret or configmap | Check `kubectl get secret -n hermesx` and `kubectl get cm -n hermesx` |

---

### Health probe failures

**Symptom:** Pods start but fail liveness or readiness probes, causing restarts or removal from endpoints.

**Diagnosis:**

```bash
# Port-forward and test manually
kubectl port-forward svc/hermesx 8080:8080 -n hermesx
curl -v http://localhost:8080/health/live
curl -v http://localhost:8080/health/ready

# Check pod probe configuration
kubectl get pod <pod-name> -n hermesx -o jsonpath='{.spec.containers[*].livenessProbe}'
kubectl get pod <pod-name> -n hermesx -o jsonpath='{.spec.containers[*].readinessProbe}'
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| Database not ready | Ensure PostgreSQL pod is `Running` before hermesx starts. Add `initContainers` or increase `initialDelaySeconds` |
| `DATABASE_URL` incorrect | Verify `env.DATABASE_URL` in values matches the PostgreSQL service address |
| Readiness probe too aggressive | Increase `readiness.initialDelaySeconds` (default: 10s) or `periodSeconds` |

---

### MinIO not ready

**Symptom:** `/health/ready` returns MinIO as unhealthy, or bootstrap Job fails with `mc: Unable to initialize new alias`.

**Diagnosis:**

```bash
# Check MinIO pod
kubectl get pod minio-0 -n hermesx

# Test MinIO health endpoint
kubectl port-forward svc/minio-api 9000:9000 -n hermesx &
curl http://localhost:9000/minio/health/live
```

**Fix:**

- Wait 30 seconds after first deploy — MinIO takes time to initialize its data directory on first start.
- Verify `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, and `MINIO_SECRET_KEY` env vars match the values in `deploy/kind/minio.yaml`.

---

### Bootstrap Job fails

**Symptom:** `hermesx-bootstrap` Job stays in `Running` or `Failed` state.

**Diagnosis:**

```bash
kubectl logs job/hermesx-bootstrap -n hermesx
```

**Common causes and fixes:**

| Cause | Fix |
|---|---|
| `hermesx` not ready | Ensure hermesx is `Running` and `/health/ready` returns 200 before applying bootstrap |
| Wrong `HERMES_ACP_TOKEN` | Ensure `HERMES_ACP_TOKEN` in `config.env` / Secret matches `env.HERMES_ACP_TOKEN` in values |
| MinIO not initialized | Wait 30s, then re-run the Job |
| Network policy blocking | Ensure the Job pod can reach `hermesx:8080` and `minio:9000` |
| Image pull failure | Ensure `alpine:3.19` is accessible or pull it manually: `docker pull alpine:3.19` |

**Re-run the Job:**

```bash
kubectl delete job hermesx-bootstrap -n hermesx
kubectl apply -k deploy/k8s/quickstart/
```

---

### ImagePullBackOff

**Symptom:** Pod stuck in `ImagePullBackOff`.

**Fix:**

```bash
# For Kind — ensure the image is loaded
kind load docker-image hermesx/hermesx-saas:local --name kind-hermes

# For Minikube — use the internal registry
minikube image load hermesx/hermesx-saas:local

# Verify image is available in the cluster node
kubectl run debug --image=hermesx/hermesx-saas:local --rm -it --restart=Never -- echo "image exists"
```

---

### HPA not scaling

**Diagnosis:**

```bash
kubectl describe hpa hermesx -n hermesx
kubectl top pods -n hermesx
kubectl describe deployment hermesx -n hermesx
```

**Common causes:**

- `metrics-server` not installed — install with `kubectl apply -f deploy/k8s/metrics-server/`
- CPU/memory not under pressure — load test to trigger scaling
- HPA max replicas reached — increase `autoscaling.maxReplicas`

---

## 13. Cleanup

### Helm uninstall

```bash
helm uninstall hermesx -n hermesx
```

### Delete namespaces

```bash
kubectl delete namespace hermesx
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

The following metrics are exposed by `hermesx` at `GET /metrics`.

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
| `HERMES_ACP_TOKEN` | Yes | — | Static bearer token for ACP admin endpoints. Store in a Secret and use a high-entropy value |
| `SAAS_API_PORT` | No | `8080` | Port for the SaaS API server |
| `SAAS_ALLOWED_ORIGINS` | No | — (CORS disabled) | CORS allowed origins. Use explicit origins in production; avoid `*` |
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
│   └── hermesx/
│       ├── Chart.yaml
│       ├── values.yaml        ← Default values
│       └── templates/
│           ├── deployment.yaml
│           └── service.yaml
├── kind/
│   ├── values.local.yaml      ← Kind/Minikube overlay
│   ├── mysql.yaml             ← MySQL StatefulSet
│   ├── redis.yaml             ← Redis deployment
│   └── rustfs.yaml            ← S3-compatible object storage
└── k8s/
    ├── quickstart/
    │   ├── kustomization.yaml
    │   ├── config.env.example  ← LLM credentials template
    │   ├── bootstrap.sh       ← Idempotent tenant seeder
    │   └── bootstrap-job.yaml
    └── gvisor-runtimeclass.yaml
```
