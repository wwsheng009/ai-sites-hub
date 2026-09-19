# NewAPI 识别与 Token 登录修复计划

## 已确认事实
- `https://api.careke.cn/api/status` 返回 HTTP 200，NewAPI 响应为 `{success, message, ...}`，`version`、`system_name` 在顶层；当前探测应支持该形态。
- 参考 `E:\projects\ai\new-api` 当前实现：PAT/Access Token 通过 `Authorization: Bearer <token>` 认证；`/api/user/self` 受 `UserAuth` 保护；密码登录成功响应 `data` 含 `access_token`，同时使用 refresh cookie。
- 当前 `newapi.Login` 只接受用户名密码，token 模式会直接返回“缺少用户名或密码”。
- 当前 `newapi.Verify` 只检查 token 是否存在，但实际请求没有附加 `Authorization` 头，必然得到未登录。
- 当前密码登录忽略登录响应中的 `data.access_token`，成功后无法保存可用于后续同步的凭据。
- 前端“Token / PAT”字段实际提交为 `access_token`，因此后端必须把 `Credentials.AccessToken` 当作 PAT 兼容处理。

## 实施
1. 增强 NewAPI 认证：统一 token 归一化/选择逻辑；token/PAT 模式调用带 Bearer 头的 `/api/user/self` 校验，成功返回并持久化 PAT/AccessToken。
2. 修正用户名密码登录响应解析，提取现代 NewAPI `data.access_token`（兼容旧形态），并在成功时返回 token；对 401/403/429、响应 message 做准确状态映射。
3. 增强 `/api/status` 识别解析，兼容顶层字段与 `data` 嵌套字段，并保留证据；避免只因响应字段形态变化判定 unknown。
4. 为认证与识别增加 httptest 单元测试，断言 Authorization 头、token 模式、登录 token 提取和两种 status 响应。
5. 运行 gofmt、相关 `go test`、全量 Go 测试及前端构建（若环境允许），检查不改动用户现有未跟踪文件。

## 风险/边界
- 不在日志、测试输出或提交中打印用户提供的真实 token。
- 不修改已有用户未提交的 `internal/adapter/sub2api/usage.go` 及临时文件。
- 浏览器 refresh cookie 仅在需要跨请求密码会话时考虑；当前 token/PAT 路径不依赖 cookie。