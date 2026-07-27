// Package serviceauth 提供服务间身份认证（Workload Identity）。
//
// 它只回答"谁在调你"——不回答"能不能调"。权限是业务的事。
//
// # 三个原子能力
//
//  1. 签票（SignTicket）：用本地 Ed25519 私钥签发身份凭证。
//  2. 验票（VerifyTicket）：用调用方的公钥验证签名。
//  3. Guard：HTTP 中间件，自动验票并注入 CallerInfo。
//
// # 接入方式（只一种）
//
//	client, _ := serviceauth.New(Config{ServiceName: "my-service"})
//	// 注册到中心（后台 sync 自动启动）
//	// 然后直接用 Post/Get/Put/Delete 调用
//
// # 设计文档
//
//	docs/serviceauth.md          — 操作手册（how）
//	docs/serviceauth-design.md   — 设计理论（why）
package serviceauth
