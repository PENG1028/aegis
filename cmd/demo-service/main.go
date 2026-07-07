// Demo service — 完整的 ServiceAuth 使用参考
//
// 这个服务展示了一个外部服务如何通过 ServiceAuth + Action API
// 管理自己的域名映射，供其他团队参考实现。
//
// 运行方式：
//
//	# 需要先启动 Aegis（监听 127.0.0.1:7380）或指定地址
//	go run cmd/demo-service/main.go
//
// # 或者指定 Aegis 地址
//
//	AEGIS_URL=http://192.168.1.100:7380 go run cmd/demo-service/main.go
//
// 流程：
//
//	1. 注册到 ServiceAuth（自动生成 Ed25519 密钥对）
//	2. 用 CallAegis 创建 HTTP 域名映射
//	3. 用 Guard 保护自己的端点
//	4. 显示拓扑依赖
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"aegis/pkg/serviceauth"
)

func main() {
	// ─── 配置 ──────────────────────────────────────────────
	serviceName := getEnv("SERVICE_NAME", "demo-service")
	servicePort := 9090
	aegisURL := getEnv("AEGIS_URL", "http://127.0.0.1:7380")
	domain := getEnv("DEMO_DOMAIN", "demo.example.com")
	targetHost := getEnv("DEMO_TARGET", "127.0.0.1")

	ctx := context.Background()

	// ─── 第 1 步：创建 SDK 客户端 ──────────────────────────
	// 声明暴露的 API，其他服务可以通过 client.Call() 调用这些端点
	client, err := serviceauth.New(serviceauth.Config{
		ServiceName: serviceName,
		ServicePort: servicePort,
		AegisURL:    aegisURL,
		APIs: []serviceauth.APIDef{
			{Name: "hello", Path: "/api/hello", Method: "GET"},
			{Name: "health", Path: "/healthz", Method: "GET"},
		},
	})
	if err != nil {
		log.Fatalf("❌ New client: %v", err)
	}
	defer client.Close()

	// ─── 第 2 步：注册到集群 ──────────────────────────────
	// 自动：生成/加载 Ed25519 密钥 → 发公钥 → 收全集群服务列表
	// 注册后自动启动后台 sync goroutine，每 30s 拉取集群变更
	if err := client.Register(ctx); err != nil {
		log.Fatalf("❌ Register: %v", err)
	}
	log.Printf("✅ 已注册为 %s (ID=%s)", serviceName, client.ServiceID())

	// ─── 第 3 步：用 Action API 创建域名映射 ──────────────
	// CallAegis 自动签 Ed25519 ticket → 发 X-Service-Ticket header
	// 不需要 admin token，不需要手动配置
	body := map[string]interface{}{
		"domain":      domain,
		"target_host": targetHost,
		"target_port": 3000,
	}
	data, _ := json.Marshal(body)
	resp, err := client.CallAegis(ctx, "/api/v1/actions/bind-http-domain", "POST", bytes.NewReader(data))
	if err != nil {
		log.Printf("⚠️  bind domain failed (may already exist): %v", err)
	} else {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		log.Printf("✅ 域名映射创建成功: %s → %s:3000", domain, targetHost)
		log.Printf("   响应: %v", result)
	}

	// ─── 第 4 步：查看自己的资源 ──────────────────────────
	// 服务可以查看自己管理的域名和路由
	resp, err = client.CallAegis(ctx, "/api/v1/my/routes", "GET", nil)
	if err != nil {
		log.Printf("⚠️  list my routes failed: %v", err)
	} else {
		var routesResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&routesResp)
		resp.Body.Close()
		routes, _ := routesResp["routes"].([]interface{})
		log.Printf("📋 我的路由 (%d 条):", len(routes))
		for _, r := range routes {
			if rt, ok := r.(map[string]interface{}); ok {
				log.Printf("   - %s (status=%v)", rt["domain"], rt["status"])
			}
		}
	}

	// ─── 第 5 步：查看拓扑依赖 ────────────────────────────
	// 查看集群中有哪些服务，以及它们之间的调用关系
	resp, err = client.CallAegis(ctx, "/api/admin/v1/service-auth/topology?window=1h", "GET", nil)
	if err != nil {
		log.Printf("⚠️  topology query failed: %v", err)
	} else {
		var topo map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&topo)
		resp.Body.Close()
		nodes, _ := topo["nodes"].([]interface{})
		edges, _ := topo["edges"].([]interface{})
		log.Printf("🔗 集群拓扑: %d 个服务, %d 条调用边", len(nodes), len(edges))
		for _, e := range edges {
			if edge, ok := e.(map[string]interface{}); ok {
				log.Printf("   %s → %s (%v 次调用)", edge["caller"], edge["target"], edge["count"])
			}
		}
	}

	// ─── 第 6 步：保护自己的端点 ──────────────────────────
	// Guard 中间件验证 X-Service-Ticket 并注入调用方信息
	mux := http.NewServeMux()

	mux.Handle("GET /api/hello", client.Guard("hello",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller := serviceauth.CallerFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":   "Hello from demo-service!",
				"caller":    caller.ServiceName,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		}),
	))

	// Health endpoint（公开，不需要 ticket）
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"name":   serviceName,
		})
	})

	// 启动 HTTP 服务
	addr := fmt.Sprintf(":%d", servicePort)
	log.Printf("🚀 启动 HTTP 服务 %s", addr)
	log.Printf("   公开端点: GET http://127.0.0.1%s/healthz", addr)
	log.Printf("   受保护端点: GET http://127.0.0.1%s/api/hello (需 X-Service-Ticket)", addr)
	log.Printf("")
	log.Printf("📖 这是外部服务的参考实现。完整的流程：")
	log.Printf("   1. New() → Register()         注册到集群")
	log.Printf("   2. CallAegis()                创建域名映射")
	log.Printf("   3. Guard()                    保护端点")
	log.Printf("   4. CallerFromContext()         获取调用方身份")
	log.Printf("")
	log.Printf("   详细文档见 docs/service-api-reference.md")
	log.Printf("   UI 参考案例见 /fabric/auth 页面")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("❌ serve: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
