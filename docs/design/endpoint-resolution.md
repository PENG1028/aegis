# Endpoint 解析规则

## 概述

每个 Service 可以有多个 Endpoint，按类型区分：

| 类型 | 优先级 | 说明 |
|------|--------|------|
| local | 1（最高） | 本地/同机部署 |
| private | 2 | 内网/私有网络 |
| public | 3（最低） | 公网地址 |

## 解析规则（固定，不支持动态评分）

```
local → private → public → fail
```

1. 读取 Service 的所有 enabled endpoints
2. 按 `local → private → public` 顺序尝试
3. 每个 endpoint 做 TCP connect 检查（2s 超时）
4. 第一个 TCP 可达的 endpoint 被选中
5. 全部不可达 → 返回 `NO_AVAILABLE_ENDPOINT`

## ResolveResult

```go
type ResolveResult struct {
    Endpoint *Endpoint
    Attempts []EndpointAttempt
}

type EndpointAttempt struct {
    EndpointID string
    Type       string
    Address    string
    Success    bool
    Message    string
    LatencyMS  int64
}
```

`ResolveResult.Attempts` 记录了所有尝试，方便 dry-run / health / diagnostics
了解为什么某个 endpoint 被选中或未被选中。

## 地址规范化

Endpoint address 支持以下格式，统一规范化为 `http://host:port`：

| 输入 | 规范化输出 |
|------|-----------|
| `127.0.0.1:3001` | `http://127.0.0.1:3001` |
| `http://127.0.0.1:3001` | `http://127.0.0.1:3001` |
| `https://10.0.0.5:443` | `https://10.0.0.5:443` |

对于 HTTP reverse_proxy，Caddy 需要的 upstream 格式为完整 URL（含 scheme）。

## 不做的事情

- 不做延迟评分 / 动态权重
- 不做健康评分 / 自动最优选择
- 不做服务发现 / service mesh
- 不做自动切换（apply 时决定，运行时不变）
