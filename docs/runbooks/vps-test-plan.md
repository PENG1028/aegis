# ServiceAuth E2E 测试方案

## VPS 登录

```
Server A (面板+网关): ssh ubuntu@<SERVER_A_IP>
Server B (后端服务):   ssh ubuntu@<SERVER_B_IP>
端口: 仅 80 TCP + 443 TCP/UDP 对外开放
Aegis API: 127.0.0.1:7380
```

## 测试边界

**测什么：** 多服务注册 → 身份稳定 → Ed25519 验票 → 服务组隔离 → 重启恢复 → 跨节点调用
**不测什么：** DNS dnsmasq（需额外配置）、透明代理 iptables（需 root）、用户 JWT 签发（业务层）

## 场景：4 个真实服务模拟一套开发基础设施

参考 aetherion (OAuth2 认证服务) 和 depotkit (Docker 基础设施管理) 的实际服务结构：

```
服务拓扑:
  ┌─────────────┐     ticket      ┌──────────────┐
  │ depotly      │ ─────────────→ │ aegis         │
  │ (基础设施)    │                │ (面板+认证中心) │
  │ :8081        │                │ :7380          │
  └──────┬───────┘                └──────┬─────────┘
         │ ticket                        │ ticket
         ↓                               ↓
  ┌─────────────┐                 ┌──────────────┐
  │ aetherion    │                 │ storage-svc   │
  │ (用户认证)    │                 │ (存储代理)     │
  │ :8082        │                 │ :8083         │
  └─────────────┘                 └──────────────┘
         │ ticket                        │ ticket
         └───────────┬───────────────────┘
                     ↓
              ┌──────────────┐
              │ monitor-svc   │  ← 被所有服务调，收集健康状态
              │ :8084         │
              └──────────────┘

服务组:
  core-services = {depotly, aetherion}     → 可以互相调 + 调 aegis
  storage-group = {storage-svc}            → 隔离的数据层
  monitor-group = {monitor-svc}            → 监控层

策略:
  core-services → * → allow
  storage-group → storage-svc → allow
  core-services → storage-svc → deny       ← 基础设施不能直接碰存储
  * → monitor-svc → allow                   ← 所有人都能上报健康
```

## Phase 1: 部署 Aegis + 验证基础设施

### 1.1 部署 Aegis 到两台 VPS
```bash
# Server A
cd /path/to/aegis
make deploy-server-a

# Server B  
make deploy-server-b
```

### 1.2 验证 Aegis 启动 + ServiceAuth 端点
```bash
ssh ubuntu@<SERVER_A_IP> "curl -s http://127.0.0.1:7380/api/healthz"
# → "OK"

ssh ubuntu@<SERVER_A_IP> "curl -s http://127.0.0.1:7380/api/admin/v1/service-auth/services"
# → {"services": []}
```

### 1.3 安装基础设施依赖
```bash
ssh ubuntu@<SERVER_A_IP> "sudo apt install -y certbot iptables dnsmasq"
ssh ubuntu@<SERVER_B_IP> "sudo apt install -y certbot iptables dnsmasq"

curl -s http://127.0.0.1:7380/api/admin/v1/infra/status
# → certbot ✓, iptables ✓, dnsmasq ✓
```

## Phase 2: 服务注册 + 身份稳定性

### 2.1 测试服务启动脚本
```go
// test-services/main.go — 引用 aegis/pkg/serviceauth SDK
package main

import (
	"context" "flag" "fmt" "log" "net/http"
	"aegis/pkg/serviceauth"
)

func main() {
	name := flag.String("name", "test-svc", "service name")
	port := flag.Int("port", 8080, "listen port")
	aegisURL := flag.String("aegis", "http://127.0.0.1:7380", "Aegis URL")
	flag.Parse()

	client, _ := serviceauth.New(serviceauth.Config{
			ServiceName: *name, AegisURL: *aegisURL,
	})
	client.Register(context.Background())
	defer client.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"` + *name + `"}`))
	})
	mux.Handle("POST /ping", client.Guard("ping", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := serviceauth.CallerFromContext(r.Context())
		w.Write([]byte(`{"pong":true,"from":"` + caller.ServiceName + `"}`))
	})))
	log.Fatal(http.ListenAndServe(":"+fmt.Sprint(*port), mux))
}
```

### 2.2 启动 4 个服务
```bash
# Server A
go run . -name=depotly -port=8081 &
go run . -name=aetherion -port=8082 &
go run . -name=storage-svc -port=8083 &

# Server B (跨节点)
go run . -name=monitor-svc -port=8084 -aegis=http://<SERVER_A_IP>:7380 &
```

### 2.3 验证注册
```bash
curl -s http://127.0.0.1:7380/api/admin/v1/service-auth/services | jq '.services | length'
# → 4

# 每个服务不同的 Ed25519 公钥
ssh ubuntu@<SERVER_A_IP> "sqlite3 /var/lib/aegis/aegis.db 'SELECT name, substr(public_key,1,20) FROM svc_auth_services'"
# → 4 行，4 个不同的 public_key
```

### 2.4 身份稳定性测试
```bash
# 重启后 identity 不变
kill $(pgrep -f "name=aetherion")
go run . -name=aetherion -port=8082 &
# → 同一个 name，service_id 不变

# 换端口后 identity 不变
kill $(pgrep -f "name=depotly")
go run . -name=depotly -port=9091 &
# → name 仍为 "depotly"，port 更新为 9091
```

## Phase 3: 验票 + 服务间调用

### 3.1 有效 ticket
```bash
# depotly 调 aetherion（SDK 自动签 Ed25519 ticket）
curl -X POST http://127.0.0.1:8082/ping \
  -H "X-Service-Ticket: $(go run sign.go -caller=depotly -target=aetherion -api=ping)"
# → {"pong":true,"from":"depotly"}
```

### 3.2 无/假/错/过期 ticket
```bash
# 无 ticket → 401
# 假 ticket → 403  
# 错误密钥 → 403
# 过期 ticket → 403
```

### 3.3 跨节点调用
```bash
# Server A → Server B 的 monitor-svc
# 需要 GatewayLink 或直连 Server B:7380
curl -s http://<SERVER_B_IP>:7380/api/admin/v1/service-auth/services
```

## Phase 4: 服务组 + 策略

### 4.1 创建服务组
```bash
curl -X POST http://127.0.0.1:7380/api/admin/v1/service-auth/groups \
  -H "Content-Type: application/json" \
  -d '{"name":"core-services","members":["depotly","aetherion"]}'

curl -X POST http://127.0.0.1:7380/api/admin/v1/service-auth/groups \
  -H "Content-Type: application/json" \
  -d '{"name":"storage-group","members":["storage-svc"]}'
```

### 4.2 创建策略
```bash
# core-services → * → allow
curl -X POST .../policies -d '{"subject":"core-services","target_service":"*","effect":"allow"}'
# core-services → storage-svc → deny（隔离）
curl -X POST .../policies -d '{"subject":"core-services","target_service":"storage-svc","effect":"deny"}'
```

### 4.3 验证隔离
```bash
# depotly（core-services）调 storage-svc → 403
# storage-svc 调 monitor-svc → 200（默认 allow）
```

## Phase 5: 容灾

### 5.1 Aegis 重启 → 验票继续
```bash
sudo systemctl restart aegis
# SDK 本地公钥+私钥缓存仍在 → ticket 签发/验证不受影响
```

### 5.2 Rebind
```bash
curl -X POST .../services/depotly/rebind -d '{"new_name":"depotly-v2"}'
# → 新密钥对，旧密钥立即失效 → 403
```

## 验收清单 (17 项)

| # | 测试项 | 预期 |
|---|--------|------|
| 1 | 4 服务全部注册 | count=4 |
| 2 | 不同 Ed25519 公钥 | 4 个不同的 public_key |
| 3 | 重启后身份不变 | 同 service_id |
| 4 | 换端口身份不变 | 同 name, port 更新 |
| 5 | 有效 ticket → 200 | pong from depotly |
| 6 | 无 ticket → 401 | |
| 7 | 假 ticket → 403 | |
| 8 | 错误密钥 → 403 | |
| 9 | 过期 ticket → 403 | |
| 10 | 跨节点调用 | A → B 成功 |
| 11 | 服务组创建 | 2 groups |
| 12 | 策略 deny 生效 | core → storage = 403 |
| 13 | 策略 allow 生效 | storage → monitor = 200 |
| 14 | default deny | 无策略 → 403 |
| 15 | Aegis 重启后验票继续 | 200 |
| 16 | Rebind 旧密钥失效 | 403 |
| 17 | 重注册 last_seen 更新 | |

## 不在此次范围
- 透明代理 iptables + DNS dnsmasq（Phase 2 单独测）
- 用户 JWT / API Key（业务层）
- 性能压测 / 多节点 distnode
