# ServiceAuth Protocol v2

多语言服务可自行实现此协议对接 ServiceAuth，无需 Go SDK。

## 注册

```
POST {gateway}/api/service-auth/v1/register
{
  "service_name": "my-service",
  "host": "10.0.1.5",
  "port": 8080,
  "node_host": "prod-1",
  "apis": [{"name": "getUser", "path": "/api/users/:id", "method": "GET"}],
  "public_key": "<base64 Ed25519 public key>"
}

Response 200:
{
  "service_id": "svc_xxx",
  "instances": [{"name": "my-service", "host": "10.0.1.5", "port": 8080, "node_host": "prod-1"}],
  "public_keys": {"my-service": "<base64>", "other-service": "<base64>"},
  "apis": [...],
  "blocklist": [...],
  "bl_version": 1, "cat_version": 1,
  "sync_interval": 30
}
```

## 同步

```
GET {gateway}/api/service-auth/v1/sync?bl_version=0&cat_version=0

Response 200 (有变更): { "blocklist": [...], "bl_version": 2, "public_keys": {...}, ... }
Response 304 (无变更): empty body
```

## 签发 Ticket（调用方本地）

```
1. 生成密钥对: Ed25519 keypair, 私钥存本地文件
2. 构造 payload: "{caller_name}:{target_name}:{api_name}:{expiry_unix}"
3. 签名: Ed25519.Sign(private_key, payload) → signature (64 bytes)
4. Ticket: base64(payload + ":" + base64(signature))
5. Header: X-Service-Ticket: <ticket>
```

## 验票（接收方本地）

```
1. 读 X-Service-Ticket header
2. base64 解码 → 按 ":" 分割 5 段: [caller, target, api, expiry, sig_b64]
3. 从本地缓存查 caller_name → public_key
4. Ed25519.Verify(public_key, payload, signature) → true/false
5. 检查 expiry > now
6. 检查 target == 本服务名
7. 检查 api 匹配
```

## 上报

```
POST {gateway}/api/service-auth/v1/report
{
  "caller_service": "my-service",
  "target_service": "other-service",
  "target_api": "getUser",
  "caller_host": "10.0.1.5",
  "target_host": "10.0.1.6",
  "allowed": true,
  "latency_ms": 42
}
```
