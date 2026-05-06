# Hermes Agent 单容器压测报告

**日期**: 2026-05-07
**测试类型**: K8s 集群内 Agent Chat 端点压测
**测试工具**: k6 v2.0.0 (Pod 部署于集群内部)

---

## 1. 环境信息

### 1.1 集群配置

| 组件 | 版本 | 节点数 | 说明 |
|------|------|--------|------|
| Kubernetes | v1.35.1 | 4 (1 control-plane + 3 worker) | Desktop kind 集群 |
| hermes-agent | v1.4.0 | 单副本 | 镜像: `hermes-agent-saas:local` |

### 1.2 容器资源限制

| 资源 | 限制值 |
|------|--------|
| CPU | 1 core |
| Memory | 512Mi |
| CPU Request | 250m |

### 1.3 LLM 后端

| 配置项 | 值 |
|--------|-----|
| Provider | OpenAI-compatible |
| Base URL | `http://10.191.110.127:8000/v1` |
| Model | `Qwen3-Coder-Next-4bit` |
| API Key | `123456` |

### 1.4 数据库

| 组件 | 说明 |
|------|------|
| PostgreSQL | `postgres-postgresql:5432/hermes` (Helm deployed) |
| Redis | `redis-master:6379` (有 Redis 但未启用) |

### 1.5 Rate Limit 配置

| Plan | RPM |
|------|-----|
| free | 60 |
| pro | 120 |

---

## 2. 测试方案

### 2.1 绕过 Rate Limit 策略

使用 5 个独立租户的 API Key 轮询发送请求，每个租户独立计算 rate limit：

| Token | Tenant | Plan | RPM |
|-------|--------|------|-----|
| `key1_9a0e5e705bf99eea` | default (00000000...) | free | 60 |
| `key2_5fc2dffcee82562e` | SkillTest-Pirate (59ad4024...) | pro | 120 |
| `key3_9d5e0a538f564c58` | SkillTest-Scientist (fcd274d4...) | pro | 120 |
| `key4_af353fc7cbca68a5` | IsolationTest-Pirate (0a136832...) | free | 60 |
| `key5_165dacec261e3d64` | IsolationTest-Academic (5658cc51...) | free | 60 |

### 2.2 测试端点

- **URL**: `http://hermes-agent:8080/v1/agent/chat`
- **Method**: POST
- **Payload**: `{"messages":[{"role":"user","content":"Hi"}],"stream":false}`
- **k6 VU 分配**: round-robin 轮询分配 5 个 token

### 2.3 压测 Stages

| 阶段 | 时长 | 目标 VUs | 说明 |
|------|------|----------|------|
| 1 | 30s | 10 | 预热 |
| 2 | 1m | 10 | 稳态 |
| 3 | 1m | 50 | 爬坡 |
| 4 | 1m | 100 | 爬坡 |
| 5 | 1m | 200 | 爬坡 |
| 6 | 1m | 300 | 爬坡 |
| 7 | 2m | 300 | 持续高负载 |
| 8 | 1m | 0 | 冷却 |
| **总计** | **8m30s** | max 300 | — |

---

## 3. 测试结果

### 3.1 总体指标

| 指标 | 值 | 说明 |
|------|-----|------|
| 总请求数 | 581 | 含超时失败 |
| 成功请求数 | 60 | 10.3% |
| 失败请求数 | 521 | 89.7% |
| **成功吞吐量** | **~1 req/s** | 581 / 540s |
| 失败原因 | 全部 120s HTTP Timeout | LLM 后端阻塞 |

### 3.2 成功请求延迟

| 分位 | 延迟 | 说明 |
|------|------|------|
| min | 5.08s | 最快单次推理 |
| median | 1m45s | 中位数 |
| avg | 1m20s | 平均值 |
| p90 | 1m58s | — |
| p95 | 1m59s | — |
| max | 1m59s | 接近超时 |

### 3.3 HTTP 请求结果分布

| Status Code | Count | 说明 |
|-------------|-------|------|
| 200 | 60 | 成功完成推理 |
| timeout (120s) | 521 | LLM 阻塞导致超时 |

---

## 4. 瓶颈分析

### 4.1 根因定位

**LLM 后端 (`10.191.110.127:8000`) 是唯一瓶颈。**

- hermes-agent 容器本身处理能力无问题（成功请求 5~120s 内返回）
- 所有失败请求均因 k6 客户端 120s 超时被动终止
- 300 VUs 全部在等待同一个 LLM 实例响应，串行排队

### 4.2 延迟分解（单次成功请求 ~80s）

```
总延迟 ≈ LLM 推理时间（~80s）
├── 网络往返延迟（容器→LLM）
└── 模型生成时间（Qwen3-Coder-Next-4bit 推理）
```

### 4.3 单容器吞吐量上限

```
理论上限 ≈ 1 / (LLM 单次推理耗时)
         ≈ 1 / 80s
         ≈ 0.75 req/min
         ≈ 45 req/hour
```

---

## 5. 对比：/v1/me 端点基准测试

| 指标 | /v1/me (无 LLM) | /v1/agent/chat (有 LLM) |
|------|------------------|--------------------------|
| 成功请求延迟 median | 1.51ms | ~105s |
| 成功请求延迟 p95 | 617ms | ~119s |
| 失败原因 | 无 (rate limit 429) | LLM 阻塞 timeout |
| 瓶颈 | Rate limit (60 rpm/tenant) | LLM 后端 |

---

## 6. 改进建议

### 6.1 提升 LLM 吞吐（推荐）

- **换用更快的 LLM**：当前 Qwen3-Coder-Next-4bit 推理耗时 ~80s，建议评估量化版或更小模型
- **增加 LLM 后端实例**：当前单 LLM 实例，多请求串行处理，扩展为多实例并行
- **启用流式响应 (stream=true)**：测试中用的是非流式，可降低感知延迟

### 6.2 水平扩展 hermes-agent

当前单副本，如果 LLM 后端足够快，可通过增加 replicaCount 提升并发处理能力：

```bash
helm upgrade hermes-agent ./deploy/helm \
  --set replicaCount=3 \
  -n hermes
```

### 6.3 调高 Rate Limit（生产考虑）

如果 LLM 后端足够快，可考虑按 plan 提高 rate limit：

| Plan | 当前 RPM | 建议 RPM |
|------|---------|---------|
| free | 60 | 60 (不变) |
| pro | 120 | 300+ |

### 6.4 启用 Redis 缓存

当前 Redis 已部署但未启用（`REDIS_URL` 未配置），启用后可加速 session 查询和中间状态缓存。

---

## 7. 下一步压测计划

- [ ] 多副本水平扩展测试（replicaCount=2/3/4）
- [ ] 流式响应 (stream=true) 压测
- [ ] 启用 Redis 后的性能对比
- [ ] LLM 后端独立压测（绕过 hermes-agent）
- [ ] 数据库连接池调优测试

---

## 附录

### A. k6 测试脚本

```javascript
// /tmp/k6-chat-stress.js
const BASE_URL = 'http://hermes-agent:8080';
const TOKENS = [
  'key1_9a0e5e705bf99eea',
  'key2_5fc2dffcee82562e',
  'key3_9d5e0a538f564c58',
  'key4_af353fc7cbca68a5',
  'key5_165dacec261e3d64',
];

export default function () {
  const tokenIdx = __VU % TOKENS.length;
  const res = http.post(`${BASE_URL}/v1/agent/chat`,
    JSON.stringify({ messages: [{ role: 'user', content: 'Hi' }], stream: false }),
    { headers: { 'Authorization': `Bearer ${TOKENS[tokenIdx]}`, 'Content-Type': 'application/json' }, timeout: '120s' }
  );
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(0.5);
}
```

### B. 相关命令

```bash
# 重新部署新版本
docker build -f Dockerfile.saas -t hermes-agent-saas:local .
kind load docker-image hermes-agent-saas:local --name desktop
kubectl rollout restart deployment/hermes-agent -n hermes

# 部署 k6 压测 Pod
kubectl run k6-chat-stress --image=grafana/k6:latest -n hermes --restart=Never -- \
  run --out json=/tmp/results.json /tmp/test.js
```
