# ServiceAuth 使用模式

## 模式 1: 纯内部服务

两个服务互调，不接受外部请求。

```go
// project-service/main.go
client, _ := serviceauth.New(serviceauth.Config{
    ServiceName: "project-service", ServicePort: 8080,
    APIs: []serviceauth.APIDef{{Name: "createProject", Path: "/api/projects", Method: "POST"}},
})
client.Register(context.Background())
defer client.Close()

mux := http.NewServeMux()
mux.Handle("POST /api/projects", client.Guard("createProject", createHandler))
http.ListenAndServe(":8080", mux)

// 调另一个服务
resp, _ := client.Call(ctx, "notification-service", "sendEmail", body)
```

## 模式 2: 用户认证服务

auth-service 对外签发 JWT，对内提供 verify 端点。

```go
// auth-service/main.go
client, _ := serviceauth.New(serviceauth.Config{
    ServiceName: "auth-service", ServicePort: 8080,
    APIs: []serviceauth.APIDef{{Name: "verifyToken", Path: "/api/internal/verify", Method: "POST"}},
})
client.Register(context.Background())

mux := http.NewServeMux()
// 对外: JWT 登录（无 ServiceAuth guard）
mux.HandleFunc("POST /api/login", loginHandler)
// 对内: 其他服务验证 token
mux.Handle("POST /api/internal/verify", client.Guard("verifyToken", verifyHandler))
http.ListenAndServe(":8080", mux)
```

另一个服务使用 auth-service：

```go
// project-service 收到用户请求 → 验证 user_token
userToken := r.Header.Get("Authorization")
resp, _ := client.Call(ctx, "auth-service", "verifyToken",
    bytes.NewReader(json.Marshal(map[string]string{"token": userToken})))
var user UserInfo; json.NewDecoder(resp.Body).Decode(&user)
// 现在 project-service 知道了用户身份，可以做业务授权
```

## 模式 3: API Key + 内部 Ticket

CI/CD 用 API Key 调 deploy-service，deploy-service 内部用 Ticket 调 notification-service。

```go
// deploy-service/main.go
client, _ := serviceauth.New(serviceauth.Config{
    ServiceName: "deploy-service", ServicePort: 8080,
    APIs: []serviceauth.APIDef{{Name: "deploy", Path: "/api/deploy", Method: "POST"}},
})
client.Register(context.Background())

mux := http.NewServeMux()
// 对外: API Key 验证（自己写）
mux.HandleFunc("POST /api/deploy", apiKeyMiddleware(deployHandler))
http.ListenAndServe(":8080", mux)

func deployHandler(w, r) {
    // API Key 通过了，现在调 notification-service
    resp, _ := client.Call(r.Context(), "notification-service", "notifyDeploy", body)
    // ...
}
```

## 模式 4: 混合认证（anyOf）

同一个端点接受多种认证方式。

```go
func main() {
    client.Register(ctx)
    mux := http.NewServeMux()
    // 混合: JWT（浏览器用户）或 Ticket（其他服务）
    mux.Handle("GET /api/projects",
        anyOf(jwtMiddleware, client.Guard("listProjects"))(listHandler))
}

func anyOf(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            for _, mw := range middlewares {
                rec := &statusRecorder{ResponseWriter: w}
                mw(http.HandlerFunc(func(w2, r2) { rec.passed = true; next.ServeHTTP(w2, r2) })).ServeHTTP(rec, r)
                if rec.passed { return }
            }
            http.Error(w, "unauthorized", 401)
        })
    }
}

func listHandler(w, r) {
    // 判断身份来源
    if caller := serviceauth.CallerFromContext(r.Context()); caller.ServiceName != "" {
        log.Printf("内部服务 %s 调用", caller.ServiceName)
    } else if user := getUserFromJWT(r); user != "" {
        log.Printf("用户 %s 调用", user)
    }
}
```

## 模式 5: 存储隔离（服务组 + 策略）

```go
// storage-service 检查调用方是否属于 storage-group
func getDataHandler(w, r) {
    caller := serviceauth.CallerFromContext(r.Context())
    if !client.InGroup(caller.ServiceName, "storage-group") {
        http.Error(w, "forbidden", 403)
        return
    }
    // 业务级隔离: 只看 caller 自己的数据
    data := loadData(caller.ServiceName)
    json.NewEncoder(w).Encode(data)
}
```

管理员配置：
```json
// PUT /api/admin/v1/service-auth/groups
{"name": "storage-group", "members": ["B", "C", "D", "E", "F"]}

// PUT /api/admin/v1/service-auth/policies  
{"subject": "storage-group", "target_service": "storage-service", "effect": "allow"}
```
