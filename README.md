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
