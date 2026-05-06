# Hermes-Agent K8s Full Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy the complete hermes-agent-go platform on local Kubernetes (Kind/Minikube) with all services, monitoring, service discovery, and production-ready health probes.

**Architecture:**
- Unified Helm chart (`deploy/helm/hermes/`) managing all application components
- Monitoring stack via kube-prometheus-stack (Prometheus + Grafana)
- Per-component health/readiness probes wired to K8s
- Ingress for external access; NodePort fallback for local
- Bootstrap Job for tenant initialization

**Tech Stack:** Kubernetes, Helm 3, Kind/Minikube, kube-prometheus-stack, PostgreSQL 16, Redis 7, MinIO, Prometheus, Grafana, Ingress-NGINX

---

## 0. Current State Audit

**Already exists:**
| Resource | Location | Status |
|---|---|---|
| PostgreSQL StatefulSet | `deploy/kind/postgres.yaml` | Isolated, no Helm integration |
| MinIO StatefulSet | `deploy/kind/minio.yaml` | Isolated, no Helm integration |
| Hermes Agent Deployment (Helm) | `deploy/helm/hermes-agent/` | Partial — no infra deps, no Redis |
| WebUI Deployment + Svc | `deploy/k8s/quickstart/webui-deployment.yaml` | Kustomize only |
| Bootstrap Job | `deploy/k8s/quickstart/bootstrap-job.yaml` | Kustomize only |
| Hermes WebUI Dockerfile | `webui/Dockerfile` | Alpine + envsubst |
| Hermes Agent Dockerfile | `Dockerfile.saas` | Multi-stage with static bundle |
| Hermes Agent K8s Dockerfile | `Dockerfile.k8s` | Slim runtime |

**Missing:**
1. Redis StatefulSet + Service
2. Full Hermes Helm chart with all deps (Postgres, Redis, MinIO) integrated
3. Prometheus metrics annotations already exist in Helm chart — but no Prometheus deployment
4. Grafana dashboards for Hermes metrics
5. Ingress or unified Service (current: scattered NodePorts)
6. Bootstrap Job integration into Helm chart
7. K8s manifests for metrics-server (HPA)
8. Unified `values.local.yaml` with all component configs
9. Namespace-level ResourceQuota / network policies
10. Redis Secret / ConfigMap
11. PersistentVolumeClaims for production-grade storage (vs current emptyDir)

---

## 1. File Map

```
deploy/
├── helm/
│   └── hermes/                          # NEW: unified chart (replaces hermes-agent/)
│       ├── Chart.yaml
│       ├── values.yaml                   # MODIFY: expand with all components
│       ├── values.local.yaml             # NEW: Kind local defaults
│       └── templates/
│           ├── _helpers.tpl
│           ├── NOTES.txt
│           ├── namespace.yaml            # NEW
│           ├── postgres/
│           │   ├── StatefulSet.yaml     # NEW (from kind/postgres.yaml)
│           │   └── Service.yaml         # NEW (headless)
│           ├── redis/
│           │   ├── StatefulSet.yaml     # NEW
│           │   ├── Service.yaml         # NEW
│           │   └── ConfigMap.yaml       # NEW
│           ├── minio/
│           │   ├── StatefulSet.yaml     # MODIFY from kind/minio.yaml
│           │   ├── Service.yaml         # MODIFY (headless + ClusterIP)
│           │   └── Secret.yaml          # MODIFY from kind/minio.yaml
│           ├── hermes-agent/
│           │   ├── Deployment.yaml      # MODIFY existing
│           │   ├── Service.yaml         # MODIFY existing
│           │   └── Ingress.yaml         # NEW
│           ├── webui/
│           │   ├── Deployment.yaml      # NEW (from k8s/quickstart)
│           │   └── Service.yaml         # NEW
│           ├── bootstrap/
│           │   ├── Job.yaml             # MODIFY existing
│           │   └── ConfigMap.yaml       # NEW (embed bootstrap.sh)
│           └── monitoring/
│               ├── Prometheus.yaml      # NEW: Prometheus sub-chart
│               ├── Grafana.yaml         # NEW: Grafana sub-chart
│               ├── ServiceMonitor.yaml  # NEW: scrpae Hermes
│               └── PrometheusRule.yaml  # NEW: alerting rules
├── k8s/
│   └── full/                            # NEW: flat K8s manifests (alternative to Helm)
│       ├── namespace.yaml
│       ├── postgres.yaml
│       ├── redis.yaml
│       ├── minio.yaml
│       ├── hermes-agent.yaml
│       ├── hermes-webui.yaml
│       ├── hermes-bootstrap.yaml
│       ├── hermes-ingress.yaml
│       ├── monitoring/
│       │   ├── prometheus.yaml
│       │   ├── grafana.yaml
│       │   ├── servicemonitor.yaml
│       │   └── prometheusrule.yaml
│       └── kustomization.yaml
├── kind/
│   └── values.local.yaml                # MODIFY: update image/name
└── README-k8s.md                        # NEW: deployment guide
```

---

## Task 1: Create Redis StatefulSet + Service + ConfigMap

**Files:**
- Create: `deploy/helm/hermes/templates/redis/StatefulSet.yaml`
- Create: `deploy/helm/hermes/templates/redis/Service.yaml`
- Create: `deploy/helm/hermes/templates/redis/ConfigMap.yaml`
- Create: `deploy/k8s/full/redis.yaml`

- [ ] **Step 1: Create Redis StatefulSet template**

File: `deploy/helm/hermes/templates/redis/StatefulSet.yaml`

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ include "hermes.fullname" . }}-redis
  labels:
    {{- include "hermes.labels" . | nindent 4 }}
    app: redis
spec:
  serviceName: {{ include "hermes.fullname" . }}-redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - name: redis
              containerPort: 6379
          command:
            - redis-server
            - /conf/redis.conf
          env:
            {{- if .Values.redis.password }}
            - name: REDIS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ include "hermes.fullname" . }}-redis-secret
                  key: REDIS_PASSWORD
            {{- end }}
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
          livenessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            exec:
              command: ["redis-cli", "ping"]
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 3
          volumeMounts:
            - name: redis-conf
              mountPath: /conf
            - name: redis-data
              mountPath: /data
      volumes:
        - name: redis-conf
          configMap:
            name: {{ include "hermes.fullname" . }}-redis-conf
  volumeClaimTemplates:
    - metadata:
        name: redis-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: {{ .Values.redis.persistence.size }}
```

- [ ] **Step 2: Create Redis ConfigMap template**

File: `deploy/helm/hermes/templates/redis/ConfigMap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "hermes.fullname" . }}-redis-conf
  labels:
    {{- include "hermes.labels" . | nindent 4 }}
data:
  redis.conf: |
    bind 0.0.0.0
    port 6379
    appendonly yes
    maxmemory 256mb
    maxmemory-policy allkeys-lru
    {{- if not .Values.redis.password }}
    requirepass ""
    {{- end }}
```

- [ ] **Step 3: Create Redis Service and Secret templates**

File: `deploy/helm/hermes/templates/redis/Service.yaml`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "hermes.fullname" . }}-redis
  labels:
    {{- include "hermes.labels" . | nindent 4 }}
spec:
  clusterIP: None
  selector:
    app: redis
  ports:
    - name: redis
      port: 6379
      targetPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "hermes.fullname" . }}-redis-lb
  labels:
    {{- include "hermes.labels" . | nindent 4 }}
spec:
  type: ClusterIP
  selector:
    app: redis
  ports:
    - name: redis
      port: 6379
      targetPort: 6379
```

File: `deploy/helm/hermes/templates/redis/Secret.yaml`

```yaml
{{- if .Values.redis.password }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "hermes.fullname" . }}-redis-secret
  labels:
    {{- include "hermes.labels" . | nindent 4 }}
type: Opaque
stringData:
  REDIS_PASSWORD: {{ .Values.redis.password }}
{{- end }}
```

- [ ] **Step 4: Create flat K8s redis.yaml manifest**

File: `deploy/k8s/full/redis.yaml`

Full standalone Redis StatefulSet + Service + ConfigMap for non-Helm deployments.

- [ ] **Step 5: Update Helm chart's Chart.yaml dependencies**

Add PostgreSQL sub-chart dependency for declarative infra management.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/hermes/templates/redis/ deploy/k8s/full/redis.yaml
git commit -m "feat(k8s): add Redis StatefulSet + Service to Helm chart"
```

---

## Task 2: Expand Hermes Helm Chart — Integrate All Components

**Files:**
- Modify: `deploy/helm/hermes/Chart.yaml`
- Modify: `deploy/helm/hermes/values.yaml`
- Modify: `deploy/helm/hermes/templates/deployment.yaml`
- Modify: `deploy/helm/hermes/templates/service.yaml`
- Create: `deploy/helm/hermes/templates/_helpers.tpl`
- Create: `deploy/helm/hermes/templates/NOTES.txt`

- [ ] **Step 1: Rewrite Chart.yaml with all component dependencies**

File: `deploy/helm/hermes/Chart.yaml`

```yaml
apiVersion: v2
name: hermes
description: Full Hermes-Agent platform on Kubernetes — Hermes Agent, WebUI, PostgreSQL, Redis, MinIO, Monitoring
type: application
version: 0.3.0
appVersion: "1.0.0"
keywords:
  - ai
  - agent
  - hermes
  - multi-tenant
  - saas
maintainers:
  - name: Hermes Team

dependencies:
  - name: postgresql
    version: "15.x.x"
    repository: "https://charts.bitnami.com/bitnami"
    condition: postgresql.enabled
  - name: prometheus
    version: "25.x.x"
    repository: "https://prometheus-community.github.io/helm-charts"
    condition: monitoring.enabled
  - name: grafana
    version: "8.x.x"
    repository: "https://grafana.github.io/helm-charts"
    condition: monitoring.enabled
```

- [ ] **Step 2: Rewrite values.yaml with all component sections**

File: `deploy/helm/hermes/values.yaml`

Complete values covering: namespace, hermes-agent image/replicas/args/env, postgres (enabled/size/credentials), redis (enabled/password/size), minio (enabled/credentials/bucket), webui (enabled/image/replicas), bootstrap (enabled/script), monitoring (enabled/prometheus/grafana), ingress (enabled/host/tls), autoscaling (enabled/minReplicas/maxReplicas/targetCPU/targetMemory).

- [ ] **Step 3: Create _helpers.tpl with consistent label/service functions**

File: `deploy/helm/hermes/templates/_helpers.tpl`

Common functions: `hermes.fullname`, `hermes.labels`, `hermes.chart`, `hermes.matchLabels`, `hermes.selectorLabels`, `hermes.redis.fullname`, `hermes.postgresql.fullname`, `hermes.minio.fullname`.

- [ ] **Step 4: Update Hermes Agent Deployment to reference all infra**

File: `deploy/helm/hermes/templates/hermes-agent/Deployment.yaml`

Inject env vars: `DATABASE_URL` (from postgresql sub-chart secret), `REDIS_URL` (redis://redis:6379), `REDIS_PASSWORD`, `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`. Add all probes: liveness `/health/live`, readiness `/health/ready`, startup probe for initialDelay 30s.

Add PodDisruptionBudget for zero-downtime updates.

- [ ] **Step 5: Update Hermes Agent Service with named ports**

File: `deploy/helm/hermes/templates/hermes-agent/Service.yaml`

Named ports: `api` (8080), `adapter` (8081), `acp` (from .Values.hermes.acpPort).

- [ ] **Step 6: Create Hermes Ingress**

File: `deploy/helm/hermes/templates/hermes-agent/Ingress.yaml`

Ingress for hermes-agent API on port 8080 with path-based routing.

- [ ] **Step 7: Create NOTES.txt with post-install instructions**

File: `deploy/helm/hermes/templates/NOTES.txt`

Show all service endpoints, default credentials, next steps for bootstrap.

- [ ] **Step 8: Commit**

```bash
git add deploy/helm/hermes/
git commit -m "feat(k8s): expand Helm chart with all components integrated"
```

---

## Task 3: Create WebUI Deployment + Service in Helm

**Files:**
- Create: `deploy/helm/hermes/templates/webui/Deployment.yaml`
- Create: `deploy/helm/hermes/templates/webui/Service.yaml`
- Create: `deploy/k8s/full/hermes-webui.yaml`

- [ ] **Step 1: Create WebUI Deployment template**

File: `deploy/helm/hermes/templates/webui/Deployment.yaml`

Image: `hermes-webui:local` (configurable via `values.yaml`). Env: `HERMES_BACKEND_URL` pointing to `{{ include "hermes.fullname" . }}-agent:{{ .Values.hermes.service.apiPort }}`. All probes: liveness/readiness HTTP GET `/` port 80. Resources: requests 50m/64Mi, limits 200m/128Mi. Init container not needed (envsubst handled at runtime).

- [ ] **Step 2: Create WebUI Service template**

File: `deploy/helm/hermes/templates/webui/Service.yaml`

Service type configurable (ClusterIP default, NodePort for local). Port 80 → targetPort 80.

- [ ] **Step 3: Create flat K8s manifest**

File: `deploy/k8s/full/hermes-webui.yaml`

Standalone Deployment + Service for non-Helm path.

- [ ] **Step 4: Commit**

```bash
git add deploy/helm/hermes/templates/webui/ deploy/k8s/full/hermes-webui.yaml
git commit -m "feat(k8s): add WebUI Deployment + Service to Helm chart"
```

---

## Task 4: Create Bootstrap Job in Helm

**Files:**
- Create: `deploy/helm/hermes/templates/bootstrap/Job.yaml`
- Create: `deploy/helm/hermes/templates/bootstrap/ConfigMap.yaml`
- Create: `deploy/k8s/full/hermes-bootstrap.yaml`

- [ ] **Step 1: Create bootstrap ConfigMap from bootstrap.sh**

File: `deploy/helm/hermes/templates/bootstrap/ConfigMap.yaml`

Embed `files: - bootstrap.sh=../../k8s/quickstart/bootstrap.sh` via ConfigMapGenerator. Disable name hash suffix.

- [ ] **Step 2: Create bootstrap Job template**

File: `deploy/helm/hermes/templates/bootstrap/Job.yaml`

Alpine 3.19 image with bash/curl/wget/python3/ca-certificates. Mount ConfigMap at `/scripts`. Env vars: `BASE_URL=http://{{ include "hermes.fullname" . }}-agent:{{ .Values.hermes.service.apiPort }}`, `MINIO_ENDPOINT={{ include "hermes.fullname" . }}-minio:9000`, credentials from secrets. `ttlSecondsAfterFinished: 300`, `restartPolicy: OnFailure`, `backoffLimit: 3`. Set `hermes.bootstrap.enabled: false` by default (run manually after first deploy).

- [ ] **Step 3: Create flat K8s manifest**

File: `deploy/k8s/full/hermes-bootstrap.yaml`

Standalone Job + ConfigMap.

- [ ] **Step 4: Commit**

```bash
git add deploy/helm/hermes/templates/bootstrap/ deploy/k8s/full/hermes-bootstrap.yaml
git commit -m "feat(k8s): add bootstrap Job to Helm chart"
```

---

## Task 5: Set Up Prometheus Monitoring + Grafana Dashboards

**Files:**
- Create: `deploy/helm/hermes/templates/monitoring/PrometheusRule.yaml`
- Create: `deploy/helm/hermes/templates/monitoring/GrafanaDashboard.yaml`
- Create: `deploy/helm/hermes/templates/monitoring/ServiceMonitor.yaml`
- Modify: `deploy/helm/hermes/values.yaml`
- Create: `deploy/k8s/full/monitoring/`
- Create: `deploy/k8s/full/kustomization.yaml`

- [ ] **Step 1: Create PrometheusRule for Hermes metrics alerting**

File: `deploy/helm/hermes/templates/monitoring/PrometheusRule.yaml`

Groups:
- `hermes`: `HermesAgentDown` (hermes_up==0 for 2min), `HermesHighErrorRate` (rate of 5xx > 5%), `HermesHighLatency` (p99 > 5s), `HermesDatabaseDown`, `HermesRedisDown`.
- `hermes-resources`: `HermesHighCPUUsage` (CPU > 80%), `HermesHighMemoryUsage` (Memory > 80%).
Labels: `prometheus: k8s`, `role: alert-rules`.

- [ ] **Step 2: Create ServiceMonitor for Hermes Prometheus scraping**

File: `deploy/helm/hermes/templates/monitoring/ServiceMonitor.yaml`

`ServiceMonitor` (monitoring.coreos.com/v1) targeting `hermes-agent` service on port `http` (8080). Path `/metrics`. Interval 15s. Namespace selector matching `{{ .Release.Namespace }}`.

- [ ] **Step 3: Create GrafanaDashboard ConfigMap**

File: `deploy/helm/hermes/templates/monitoring/GrafanaDashboard.yaml`

ConfigMap with `grafana_dashboard` provider. JSON dashboard covering:
- Panel 1: Hermes Up (up metric, single stat)
- Panel 2: Request Rate (requests_total by status_code, rate 1m)
- Panel 3: Request Latency (p50/p95/p99 histogram)
- Panel 4: Database Query Duration (hermes_pg_query_duration_seconds)
- Panel 5: Redis Hit Rate (cache hits vs misses)
- Panel 6: Pod CPU/Memory Usage
- Panel 7: LLM API Latency (if exposed via custom metrics)
- All panels: Hermes-specific branding, consistent dark theme colors

- [ ] **Step 4: Configure kube-prometheus-stack sub-chart in values**

File: `deploy/helm/hermes/values.yaml` (monitoring section)

```yaml
prometheus:
  enabled: true
  prometheus:
    retention: 15d
    prometheusSpec:
      replicas: 1
      evaluationInterval: 30s
      scrapeInterval: 15s
      ruleSelectorNilUsesHelmOptions: false
      serviceMonitorSelectorNilUsesHelmOptions: false
      podMonitorSelectorNilUsesHelmOptions: false
      probeSelectorNilUsesHelmOptions: false

grafana:
  enabled: true
  adminPassword: "prometheus"
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
        - name: hermes
          folder: Hermes
          type: file
          options:
            path: /var/lib/grafana/dashboards/hermes
```

- [ ] **Step 5: Create flat K8s monitoring manifests**

File: `deploy/k8s/full/monitoring/prometheus.yaml` — Prometheus CRD + PodMonitor
File: `deploy/k8s/full/monitoring/grafana.yaml` — Grafana Deployment + Service
File: `deploy/k8s/full/monitoring/servicemonitor.yaml` — Hermes ServiceMonitor
File: `deploy/k8s/full/monitoring/prometheusrule.yaml` — Prometheus alerting rules
File: `deploy/k8s/full/monitoring/grafanadashboard.yaml` — Grafana dashboard ConfigMap

- [ ] **Step 6: Create Kustomization for flat manifest deployment**

File: `deploy/k8s/full/kustomization.yaml`

Kustomize overlay referencing all resources in `deploy/k8s/full/`. CommonLabels for all resources. ConfigMapGenerator for bootstrap.sh. SecretGenerator for credentials.

- [ ] **Step 7: Commit**

```bash
git add deploy/helm/hermes/templates/monitoring/ deploy/k8s/full/monitoring/ deploy/k8s/full/kustomization.yaml
git commit -m "feat(k8s): add Prometheus monitoring + Grafana dashboards"
```

---

## Task 6: Create Ingress, HPA, and Namespace Resources

**Files:**
- Create: `deploy/helm/hermes/templates/_namespace.yaml`
- Create: `deploy/helm/hermes/templates/hermes-agent/Ingress.yaml`
- Create: `deploy/helm/hermes/templates/hermes-agent/PodDisruptionBudget.yaml`
- Create: `deploy/helm/hermes/templates/hermes-agent/HorizontalPodAutoscaler.yaml`
- Create: `deploy/k8s/full/hermes-ingress.yaml`
- Create: `deploy/k8s/full/hermes-hpa.yaml`

- [ ] **Step 1: Create Ingress for Hermes Agent + WebUI**

File: `deploy/helm/hermes/templates/hermes-agent/Ingress.yaml`

IngressClassName: nginx (configurable). Host: `{{ .Values.ingress.host }}`. Paths:
- `/api` → hermes-agent:8080
- `/v1` → hermes-agent:8080
- `/metrics` → hermes-agent:8080
- `/` → hermes-webui:80 (WebUI fallback)

Annotations: `nginx.ingress.kubernetes.io/proxy-body-size: "50m"`, `nginx.ingress.kubernetes.io/proxy-read-timeout: "300"`, cert-manager annotation if TLS enabled.

- [ ] **Step 2: Create PodDisruptionBudget**

File: `deploy/helm/hermes/templates/hermes-agent/PodDisruptionBudget.yaml`

`maxUnavailable: 1` for hermes-agent deployment to ensure at least 1 pod available during updates.

- [ ] **Step 3: Create HorizontalPodAutoscaler**

File: `deploy/helm/hermes/templates/hermes-agent/HorizontalPodAutoscaler.yaml`

Scale target: hermes-agent deployment. MinReplicas: from values (default 1), MaxReplicas: from values (default 5). CPU target: 70%, Memory target: 80% (if metrics-server available).

- [ ] **Step 4: Create flat K8s ingress and HPA manifests**

Files: `deploy/k8s/full/hermes-ingress.yaml`, `deploy/k8s/full/hermes-hpa.yaml`

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/hermes/templates/hermes-agent/Ingress.yaml
git add deploy/helm/hermes/templates/hermes-agent/PodDisruptionBudget.yaml
git add deploy/helm/hermes/templates/hermes-agent/HorizontalPodAutoscaler.yaml
git add deploy/k8s/full/hermes-ingress.yaml
git add deploy/k8s/full/hermes-hpa.yaml
git commit -m "feat(k8s): add Ingress, HPA, and PodDisruptionBudget"
```

---

## Task 7: Create values.local.yaml for Kind/Minikube

**Files:**
- Create: `deploy/helm/hermes/values.local.yaml`
- Modify: `deploy/kind/values.local.yaml`

- [ ] **Step 1: Create Kind/Minikube values file**

File: `deploy/helm/hermes/values.local.yaml`

```yaml
namespace: hermes

hermes:
  image:
    repository: hermes-agent-saas
    tag: local
    pullPolicy: Never
  replicaCount: 2
  args:
    - saas-api
  env:
    DATABASE_URL: "postgres://hermes:hermes@{{ include "hermes.fullname" . }}-postgresql:5432/hermes?sslmode=disable"
    HERMES_ACP_TOKEN: "dev-token-change-in-production"
    SAAS_API_PORT: "8080"
    SAAS_ALLOWED_ORIGINS: "*"
    SAAS_STATIC_DIR: "/static"
    HERMES_API_PORT: "8081"
    HERMES_API_KEY: "dev-api-key-change-in-production"
    LLM_API_URL: "http://10.191.110.127:8000/v1"
    LLM_API_KEY: "sk-your-api-key-here"
    LLM_MODEL: "MiniMax-M2.7-highspeed"
  probes:
    startup:
      enabled: true
      path: /health/live
      initialDelaySeconds: 10
      periodSeconds: 5
      failureThreshold: 30
    liveness:
      path: /health/live
      initialDelaySeconds: 5
      periodSeconds: 10
    readiness:
      path: /health/ready
      initialDelaySeconds: 10
      periodSeconds: 15

postgres:
  enabled: true
  persistence:
    enabled: false  # emptyDir for local dev
  auth:
    database: hermes
    username: hermes
    password: hermes-dev-password

redis:
  enabled: true
  password: ""
  persistence:
    size: 1Gi

minio:
  enabled: true
  auth:
    rootUser: hermes-minio
    rootPassword: hermes-minio-password
  persistence:
    size: 2Gi

webui:
  enabled: true
  image:
    repository: hermes-webui
    tag: local
    pullPolicy: Never

bootstrap:
  enabled: false  # Run manually after first deploy

monitoring:
  enabled: true
  prometheus:
    retention: 7d
  grafana:
    adminPassword: "admin"

ingress:
  enabled: false  # NodePort for local Kind
  host: hermes.local

autoscaling:
  enabled: false  # Enable for multi-replica production

service:
  type: NodePort  # NodePort for local Kind access
  apiPort: 30080  # hermes-agent NodePort
  adapterPort: 30081  # hermes-api adapter NodePort
```

- [ ] **Step 2: Update Kind values.local.yaml**

File: `deploy/kind/values.local.yaml`

Align with Hermes Helm chart's image names and ports.

- [ ] **Step 3: Commit**

```bash
git add deploy/helm/hermes/values.local.yaml deploy/kind/values.local.yaml
git commit -m "feat(k8s): add local Kind values for Hermes Helm chart"
```

---

## Task 8: Create Deployment Guide README

**Files:**
- Create: `deploy/README-k8s.md`
- Modify: `deploy/k8s/quickstart/config.env.example`

- [ ] **Step 1: Create comprehensive K8s deployment guide**

File: `deploy/README-k8s.md`

Sections:
1. **Prerequisites** — Kind/Minikube, Helm 3, kubectl, docker
2. **Quick Start (Helm)** — 5 commands to full deployment
3. **Quick Start (Kustomize)** — Flat manifests alternative
4. **Service Access** — NodePort ranges, port mapping table
5. **Prometheus/Grafana Access** — Port-forwards, default credentials
6. **Ingress Setup** — Enable ingress controller, configure /etc/hosts
7. **Monitoring** — What metrics are collected, dashboard guide
8. **Health Probes** — Probe endpoint reference (`/health/live`, `/health/ready`, `/metrics`)
9. **Bootstrap** — How to run bootstrap Job manually
10. **Customization** — Override values, external infra, TLS
11. **Troubleshooting** — Common issues and fixes
12. **Cleanup** — How to uninstall everything

- [ ] **Step 2: Commit**

```bash
git add deploy/README-k8s.md
git commit -m "docs(k8s): add comprehensive K8s deployment guide"
```

---

## Task 9: Create metrics-server for HPA Memory Metric

**Files:**
- Create: `deploy/k8s/full/metrics-server.yaml`
- Modify: `deploy/k8s/full/kustomization.yaml`

- [ ] **Step 1: Create metrics-server Deployment for HPA memory-based scaling**

File: `deploy/k8s/full/metrics-server.yaml`

Standard metrics-server v0.7 deployment with `kubectl top` support. API server in-cluster communication, TLS bootstrap.

- [ ] **Step 2: Add metrics-server to kustomization**

- [ ] **Step 3: Commit**

```bash
git add deploy/k8s/full/metrics-server.yaml deploy/k8s/full/kustomization.yaml
git commit -m "feat(k8s): add metrics-server for HPA resource metrics"
```

---

## Task 10: Verification — End-to-End Smoke Test

**Files:**
- Create: `deploy/k8s/smoke-test.sh`

- [ ] **Step 1: Create smoke test script**

File: `deploy/k8s/smoke-test.sh`

Run after `helm install` or `kubectl apply -k`:
1. Wait for all pods to be Ready
2. Curl `/health/live` → expect 200
3. Curl `/health/ready` → expect 200 (checks DB/Redis/MinIO connectivity)
4. Curl `/metrics` → expect Prometheus metrics
5. Curl WebUI → expect 200
6. Check Prometheus targets → hermes-agent should be UP
7. Verify all PVCs are Bound (if persistence enabled)
8. Check HPA status

Exit 0 on all pass, exit 1 on any failure.

- [ ] **Step 2: Commit**

```bash
git add deploy/k8s/smoke-test.sh
git commit -m "test(k8s): add smoke test script for K8s deployment"
```

---

## Self-Review Checklist

| Requirement | Task | Status |
|---|---|---|
| Redis StatefulSet + Service | Task 1 | ☐ |
| Hermes Agent with all infra env vars | Task 2 | ☐ |
| WebUI Deployment + Service | Task 3 | ☐ |
| Bootstrap Job in Helm | Task 4 | ☐ |
| Prometheus ServiceMonitor + PrometheusRule | Task 5 | ☐ |
| Grafana Dashboard ConfigMap | Task 5 | ☐ |
| Ingress for API + WebUI | Task 6 | ☐ |
| HPA (CPU + Memory) | Task 6 | ☐ |
| PodDisruptionBudget | Task 6 | ☐ |
| values.local.yaml for Kind | Task 7 | ☐ |
| Deployment README | Task 8 | ☐ |
| metrics-server for HPA | Task 9 | ☐ |
| Smoke test script | Task 10 | ☐ |
| Flat K8s manifests (non-Helm path) | Tasks 1-6 | ☐ |
| Kustomization for flat manifests | Task 5 | ☐ |

---

## Deployment Commands Reference

### Helm (recommended)

```bash
# 1. Build Helm dependencies
cd deploy/helm/hermes
helm dependency build

# 2. Install with local values
helm install hermes ./hermes \
  -f ./values.local.yaml \
  --namespace hermes \
  --create-namespace \
  --wait

# 3. Port-forward for access
kubectl port-forward svc/hermes-agent 8080:8080 -n hermes
kubectl port-forward svc/hermes-webui 3000:80 -n hermes
kubectl port-forward svc/prometheus 9090:9090 -n hermes
kubectl port-forward svc/grafana 3000:3000 -n hermes

# 4. Run bootstrap (optional)
helm upgrade hermes ./hermes -f ./values.local.yaml --set bootstrap.enabled=true -n hermes
kubectl wait --for=condition=complete job/hermes-bootstrap --timeout=120s -n hermes

# 5. Run smoke test
bash deploy/k8s/smoke-test.sh

# Uninstall
helm uninstall hermes -n hermes
kubectl delete ns hermes
```

### Kustomize (alternative)

```bash
# Copy and edit config
cp deploy/k8s/quickstart/config.env.example deploy/k8s/full/config.env
# Edit config.env with your LLM credentials

# Apply all resources
kubectl apply -k deploy/k8s/full/
```

---

## Appendix: Health Probe Reference

| Endpoint | Method | Purpose | Returns |
|---|---|---|---|
| `/health/live` | GET | K8s liveness probe — process alive | `{"status":"alive"}` |
| `/health/ready` | GET | K8s readiness probe — all deps healthy | `{"status":"ready","database":"ok","redis":"ok","minio":"ok"}` |
| `/metrics` | GET | Prometheus metrics | Prometheus text format |
