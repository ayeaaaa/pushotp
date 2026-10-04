# pushotp

通过 pushplus（微信）或 Telegram 发送动态验证码并校验的 Go 模块，可选签发短期 HMAC 令牌。支持多接收人、内存/SQLite 存储，附带可选 HTTP 服务入口。

## 库用法

```go
package main

import (
	"context"
	"fmt"
	"time"

	"pushotp"
)

func main() {
	v, err := pushotp.New(pushotp.Config{
		Code: pushotp.CodeConfig{
			Length:      6,
			TTL:         5 * time.Minute,
			Cooldown:    time.Minute,
			MaxAttempts: 5,
			MaxPerHour:  10,
		},
		Receivers: []pushotp.ReceiverConfig{{
			Name:     "admin",
			Channel:  "pushplus",
			Pushplus: pushotp.PushplusConfig{Token: "your-token"},
		}},
	})
	if err != nil {
		panic(err)
	}
	defer v.Close()

	tk, err := v.Send(context.Background(), pushotp.SendRequest{Receiver: "admin", Scene: "login"})
	if err != nil {
		panic(err)
	}
	fmt.Println("ticket:", tk.ID)

	res, err := v.Verify(context.Background(), pushotp.VerifyRequest{TicketID: tk.ID, Code: "123456"})
	if err != nil {
		panic(err)
	}
	fmt.Println("verified:", res.OK)
}
```

配置字段零值自动填充安全默认值；`issuer.enabled=true` 时 `Verify` 返回 `res.Token`，后续请求用 `v.VerifyToken(token)` 验签。

## HTTP 服务

```bash
cd cmd/pushotpd
cp config.example.yaml config.yaml
go run . -config config.yaml
```

| 方法 | 路径 | 请求 | 响应 |
|---|---|---|---|
| POST | `/api/v1/send` | `{"receiver","scene","ttl?","length?","data?"}` | `{"ticket_id","expires_in"}` |
| POST | `/api/v1/verify` | `{"ticket_id","code"}` | `{"ok","token?","expires_at?"}` |
| GET | `/api/v1/health` | - | `{"status":"ok"}` |

配置 `server.api_key` 后，`/send` 与 `/verify` 需要 `Authorization: Bearer <key>`；`/health` 始终公开。

## 配置参考

配置文件为 YAML，字段如下：

| 字段 | 说明 | 默认 |
|---|---|---|
| `server.addr` | HTTP 监听地址 | `:8080` |
| `server.api_key` | Bearer 鉴权密钥；为空则不鉴权 | 空（不鉴权） |
| `storage.type` | 存储类型：`memory` 或 `sqlite` | `memory` |
| `storage.sqlite.path` | SQLite 文件路径（`type: sqlite` 时必填） | - |
| `code.length` | 验证码位数（4-8） | `6` |
| `code.ttl` | 验证码有效期，Go duration 字符串（如 `5m`、`30s`） | `5m` |
| `code.cooldown` | 同一接收人两次发送的最小间隔 | `60s` |
| `code.max_attempts` | 单张票据最大校验次数 | `5` |
| `code.max_per_hour` | 每接收人每小时发送上限 | `10` |
| `issuer.enabled` | 是否在校验成功后签发 HMAC 令牌 | `false` |
| `issuer.secret` | 令牌签名密钥（启用时必填） | - |
| `issuer.ttl` | 令牌有效期，Go duration 字符串（如 24h） | `24h` |
| `receivers[].name` | 接收人名称，调用时按名指定 | 必填 |
| `receivers[].channel` | 渠道：`pushplus` 或 `telegram` | 必填 |
| `receivers[].pushplus.token` | pushplus token | pushplus 必填 |
| `receivers[].telegram.bot_token` | Telegram Bot Token | telegram 必填 |
| `receivers[].telegram.chat_id` | Telegram 会话 ID | telegram 必填 |
| `receivers[].template` | 消息模板，留空用默认文案 | 空 |
| `receivers[].config` | 自定义（第三方）渠道的凭据键值，透传给已注册渠道 | 空 |

模板内置变量：`{code}`、`{ttl}`、`{scene}`、`{receiver}`；自定义变量（如 `{app}`）通过接口/HTTP 请求的 `data` 字段传入。

## 安全默认值

| 项 | 默认 |
|---|---|
| 验证码长度 | 6 位数字（4-8 可配） |
| 有效期 | 5 分钟 |
| 发送冷却 | 60 秒 |
| 最大尝试 | 5 次 |
| 每接收人配额 | 10 次/小时 |
| 令牌有效期 | 24 小时 |

验证码只存加盐 SHA-256 哈希；恒定时间比较；一次性使用；发送失败不占配额。

## 存储

- `memory`：默认，重启后验证码失效
- `sqlite`：单文件持久化（`modernc.org/sqlite`，纯 Go 无 CGO），适合需要审计记录或重启保留的场景

## 依赖

Go 1.22+；仅 `modernc.org/sqlite` 与 `gopkg.in/yaml.v3`（后者仅 HTTP 入口使用）。
