# TG-FileStreamBot 改造更新日志

## 2026-05-18 - 基于 Redis 的文件名无感安全重定向路由改造

本次改造在完全保留并维护项目原生 `/stream/:messageID` 接口的安全哈希校验（Hash Validation）的前提下，新增了基于 Redis 文件名路由的 `/v/:filename` 直链端点，并在后台实现自动提取后缀、获取 Telegram 消息、动态计算 Hash 及 302 重定向。

---

### 🛠️ 改造工作总结

1. **引入 Go-Redis 客户端依赖**
   - 在项目根目录下成功添加并拉取了 Go Redis 官方客户端依赖库 `github.com/redis/go-redis/v9`。

2. **保留并还原原生 Hash 鉴权校验（安全性设计）**
   * 修改文件：[stream.go](internal/routes/stream.go)
   * 功能：在 `getStreamRoute` 中完全保留/恢复了原生的 `hash` 鉴权与校验逻辑。所有外部针对原生流媒体接口 `/stream/:messageID` 的直连请求，若无正确 `?hash=` 参数都将被安全拦截并返回 `400 Bad Request`，保障了防爬防猜测的安全机制。

3. **新增智能文件名路由与动态 Hash 重定向**
   * 修改文件：[stream.go](internal/routes/stream.go)
   * 路由：在 `LoadHome` 方法中注册了专属的 `/v/:filename` 访问路由。
   * 实现逻辑：
     1. **后缀剥离与兼容**：获取 URL 参数 `:filename`（如 `S0107.mp4` 或 `S0107`），通过 `filepath.Ext` 动态去除任何文件后缀，得到纯视频编号 `S0107`。
     2. **Redis 映射检索**：在 Redis 中查询 `vid:S0107`，以 O(1) 的超高效率获取对应的 Telegram Message ID。
     3. **动态计算安全 Hash**：在服务端内部，获取该 Message ID 对应的 Telegram 文件元数据（`FileName`, `FileSize`, `MimeType`, `ID`），动态为该文件计算出绝对正确且合法的 Short Hash。
     4. **302 自动重定向**：将客户端通过 `302 Found` 优雅且无感地重定向至包含正确 Hash 的原生播放地址 `/stream/:messageID?hash=xxx`，实现无缝拉流播放。

---

### 🚀 效果与接口流程展示

```mermaid
sequenceDiagram
    autonumber
    actor Client as 客户端浏览器/播放器
    participant Server as Go 视频播放网关
    participant Redis as Redis 数据库
    participant TG as Telegram API

    Client->>Server: 1. 请求直链 /v/S0107.mp4 (无 Hash)
    Note over Server: 后台剥离后缀得到纯 Key: vid:S0107
    Server->>Redis: 2. 查询 vid:S0107 的 Message ID
    Redis-->>Server: 返回 "1524"
    Server->>TG: 3. 拉取消息 1524 的文件元数据
    TG-->>Server: 返回文件详情 (Mime, 大小, 文件ID 等)
    Note over Server: 计算出该文件专属的合法 Hash: abc123xyz
    Server-->>Client: 4. 返回 302 Found 重定向到 /stream/1524?hash=abc123xyz
    Client->>Server: 5. 请求 /stream/1524?hash=abc123xyz
    Note over Server: 校验 Hash 正确，开始高性能拉流
    Server-->>Client: 6. 极速串流视频数据 (200 / 206 Partial Content)
```

#### 请求流程示例：
* **公开直链**：`http://127.0.0.1:8080/v/S0107.mp4` (或不带后缀 `http://127.0.0.1:8080/v/S0107`)
* **Redis 解析与计算**：根据 `vid:S0107` 提取到 `messageID` = `1524`，后台算出有效哈希 `hash` = `abc123xyz`。
* **重定向结果**：自动重定向至安全直链 `http://127.0.0.1:8080/stream/1524?hash=abc123xyz` 并瞬间起播。
