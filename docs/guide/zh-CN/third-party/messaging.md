# 消息通知

EgoAdmin 提供多渠道消息通知能力，支持邮件、短信、企业微信、钉钉和 Slack 等渠道的统一发送。

## 概述

消息通知系统采用统一接口 + 渠道适配器的架构模式。业务代码通过统一的通知接口发送消息，底层根据渠道类型路由到对应的适配器。异步消息通过 AsyncQ 组件实现非阻塞投递。

架构层次：

| 层次 | 职责 | 位置 |
|------|------|------|
| 接口层 | 统一通知接口，定义消息结构 | `internal/component/notify/` |
| 适配层 | 各渠道的发送适配器 | `internal/platform/notify/` |
| 异步层 | 消息队列投递，失败重试 | `internal/component/asyncq/` |

支持的消息渠道：

| 渠道 | 协议 | 适用场景 |
|------|------|----------|
| Email | SMTP | 注册确认、密码重置、报表通知 |
| SMS（阿里云） | HTTP API | 验证码、订单状态变更 |
| SMS（腾讯云） | HTTP API | 验证码、营销通知 |
| 企业微信 | Webhook | 内部告警、审批通知 |
| 钉钉 | Webhook | 内部告警、工单通知 |
| Slack | Webhook | 开发团队通知、CI/CD 告警 |

::: tip 设计原则
消息通知是异步的、尽力投递的。不保证所有渠道 100% 到达，但通过重试和状态跟踪最大化投递成功率。
:::

## 核心用法

### AsyncQ 异步队列

AsyncQ 是消息通知的异步投递基础，基于 [asynq](https://github.com/hibiken/asynq) 构建，使用 Redis 作为消息存储：

```go
// internal/component/asyncq/interface.go

type AsyncqInterface interface {
    // 客户端方法
    ClientInterface

    // 服务端方法
    ServerInterface

    // 通用方法
    Close() error
    Health(ctx context.Context) error
}

type ClientInterface interface {
    Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
    EnqueueIn(ctx context.Context, delay time.Duration, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
    EnqueueAt(ctx context.Context, processAt time.Time, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type ServerInterface interface {
    RegisterHandler(pattern string, handler asynq.Handler)
    RegisterHandlerFunc(pattern string, handler func(context.Context, *asynq.Task) error)
    Start() error
    Stop() error
    Shutdown() error
}
```

Wire 注入：

```go
// internal/component/asyncq/provider.go
var ProviderSet = wire.NewSet(
    NewAsyncqComponent,
)
```

### 发送消息

业务代码入队消息任务：

```go
type NotifyPayload struct {
    Channel  string            `json:"channel"`  // email, sms, wechat_work, dingtalk, slack
    To       string            `json:"to"`        // 接收人
    Subject  string            `json:"subject"`   // 主题（邮件）
    Body     string            `json:"body"`      // 正文
    Template string            `json:"template"`  // 模板名
    Vars     map[string]string `json:"vars"`      // 模板变量
}

func (s *NotifyService) SendEmail(ctx context.Context, to, subject, body string) error {
    payload, _ := json.Marshal(NotifyPayload{
        Channel: "email",
        To:      to,
        Subject: subject,
        Body:    body,
    })
    _, err := s.asyncq.Enqueue(ctx, asynq.NewTask("notify:email", payload))
    return err
}
```

### 注册消息处理器

在服务端注册各渠道的处理函数：

```go
func (s *Server) initNotifyHandlers() {
    // 邮件通知
    s.asyncq.RegisterHandlerFunc("notify:email", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.emailAdapter.Send(ctx, payload)
    })

    // 短信通知
    s.asyncq.RegisterHandlerFunc("notify:sms", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.smsAdapter.Send(ctx, payload)
    })

    // 企业微信机器人
    s.asyncq.RegisterHandlerFunc("notify:wechat_work", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.wechatWorkAdapter.Send(ctx, payload)
    })
}
```

### 延迟发送

使用 AsyncQ 的 `EnqueueIn` 实现延迟投递：

```go
// 5 分钟后发送提醒
func (s *NotifyService) SendReminder(ctx context.Context, userID, message string) error {
    payload, _ := json.Marshal(NotifyPayload{
        Channel: "wechat_work",
        To:      userID,
        Body:    message,
    })
    _, err := s.asyncq.EnqueueIn(ctx, 5*time.Minute, asynq.NewTask("notify:wechat_work", payload))
    return err
}
```

### 定时发送

使用 `EnqueueAt` 在指定时间发送：

```go
// 明天上午 9 点发送日报提醒
tomorrow9AM := time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
_, err := s.asyncq.EnqueueAt(ctx, tomorrow9AM, asynq.NewTask("notify:email", payload))
```

## 配置示例

### AsyncQ 组件

```toml
[component.asyncq]
redisAddr = "127.0.0.1:6380"
redisPassword = ""
redisDB = 0
enableClient = true
enableServer = true
concurrency = 10
maxRetry = 3
retryDelayFunc = "exponential"
taskTimeout = "30s"
enableAccessLog = false
slowLogThreshold = "1s"
enableHealthCheck = true

# 队列优先级（数字越大优先级越高）
[component.asyncq.queues]
critical = 6
default = 3
low = 1
```

### 邮件（SMTP）

```toml
[client.smtp]
host = "smtp.example.com"
port = 465
username = "noreply@example.com"
password = "smtp-password"
from = "EgoAdmin <noreply@example.com>"
ssl = true
```

### 短信（阿里云）

```toml
[client.sms.aliyun]
accessKeyId = "your-access-key-id"
accessKeySecret = "your-access-key-secret"
signName = "EgoAdmin"
templateCode = "SMS_123456789"
```

### 短信（腾讯云）

```toml
[client.sms.tencent]
secretId = "your-secret-id"
secretKey = "your-secret-key"
appId = "1400000000"
signName = "EgoAdmin"
templateId = "123456"
```

### 企业微信机器人

```toml
[client.wechat_work]
webhookUrl = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=your-key"
```

### 钉钉机器人

```toml
[client.dingtalk]
webhookUrl = "https://oapi.dingtalk.com/robot/send?access_token=your-token"
secret = "SEC..."
```

### Slack

```toml
[client.slack]
webhookUrl = "https://hooks.slack.com/services/T00/B00/xxxxx"
channel = "#alerts"
username = "EgoAdmin Bot"
```

### 环境变量覆盖

```bash
# AsyncQ
EGOADMIN_COMPONENT_ASYNCQ_REDISADDR=redis.prod.example.com:6380
EGOADMIN_COMPONENT_ASYNCQ_REDISPASSWORD=prod-redis-password

# SMTP
EGOADMIN_CLIENT_SMTP_HOST=smtp.prod.example.com
EGOADMIN_CLIENT_SMTP_PASSWORD=prod-smtp-password

# 企业微信
EGOADMIN_CLIENT_WECHAT_WORK_WEBHOOKURL=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=prod-key
```

## 实战示例

### 注册验证码发送

```go
func (s *UserService) SendRegisterCode(ctx context.Context, phone string) error {
    code := generateCode()

    // 存储验证码到 Redis（5 分钟过期）
    s.redis.Set(ctx, "register:code:"+phone, code, 5*time.Minute)

    // 异步发送短信
    payload, _ := json.Marshal(NotifyPayload{
        Channel: "sms",
        To:      phone,
        Template: "register_code",
        Vars:     map[string]string{"code": code},
    })
    _, err := s.asyncq.Enqueue(ctx, asynq.NewTask("notify:sms", payload))
    return err
}
```

### 多渠道告警

当系统检测到异常时，同时通知多个渠道：

```go
func (s *MonitorService) Alert(ctx context.Context, alert *Alert) error {
    // 构造消息
    subject := fmt.Sprintf("[告警] %s - %s", alert.Level, alert.Title)
    body := fmt.Sprintf("服务: %s\n详情: %s\n时间: %s",
        alert.Service, alert.Detail, alert.Time.Format(time.RFC3339))

    // 并行发送到多个渠道
    channels := []string{"email", "wechat_work", "dingtalk"}
    for _, ch := range channels {
        payload, _ := json.Marshal(NotifyPayload{
            Channel: ch,
            To:      getChannelRecipient(ch, alert),
            Subject: subject,
            Body:    body,
        })
        // 使用 critical 队列确保高优先级
        _, err := s.asyncq.Enqueue(ctx,
            asynq.NewTask("notify:"+ch, payload),
            asynq.Queue("critical"),
        )
        if err != nil {
            slog.Error("enqueue alert failed", "channel", ch, "error", err)
        }
    }
    return nil
}
```

### 模板消息

使用模板系统统一管理消息格式：

```go
type TemplateEngine struct {
    templates map[string]*template.Template
}

func (e *TemplateEngine) Render(templateName string, vars map[string]string) (string, error) {
    tmpl, ok := e.templates[templateName]
    if !ok {
        return "", fmt.Errorf("template not found: %s", templateName)
    }
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, vars); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// 处理器中使用模板
func (h *NotifyHandler) HandleEmail(ctx context.Context, task *asynq.Task) error {
    var payload NotifyPayload
    json.Unmarshal(task.Payload(), &payload)

    if payload.Template != "" {
        rendered, err := h.tmplEngine.Render(payload.Template, payload.Vars)
        if err != nil {
            return err
        }
        payload.Body = rendered
    }

    return h.emailAdapter.Send(ctx, payload)
}
```

### 状态跟踪

在 MySQL 中记录消息发送状态，支持失败重试和审计：

```go
type MessageLog struct {
    ID        uint64     `gorm:"primaryKey"`
    Channel   string     `gorm:"index;size:32"`
    To        string     `gorm:"size:255"`
    Subject   string     `gorm:"size:255"`
    Status    string     `gorm:"size:16;index"` // pending, sent, failed
    Retry     int
    Error     string     `gorm:"type:text"`
    SentAt    *time.Time
    CreatedAt time.Time
}

// 发送前记录
func (h *NotifyHandler) beforeSend(ctx context.Context, payload *NotifyPayload) (*MessageLog, error) {
    log := &MessageLog{
        Channel: payload.Channel,
        To:      payload.To,
        Subject: payload.Subject,
        Status:  "pending",
    }
    err := h.db.Create(log).Error
    return log, err
}

// 发送后更新
func (h *NotifyHandler) afterSend(ctx context.Context, log *MessageLog, err error) {
    if err != nil {
        h.db.Model(log).Updates(map[string]interface{}{
            "status": "failed",
            "error":  err.Error(),
            "retry":  gorm.Expr("retry + 1"),
        })
    } else {
        now := time.Now()
        h.db.Model(log).Updates(map[string]interface{}{
            "status":  "sent",
            "sent_at": &now,
        })
    }
}
```

## 工作原理

### 消息投递流程

```text
业务代码
  |
  v
asyncq.Enqueue(task)
  |
  v
Redis Queue（按优先级分区：critical > default > low）
  |
  v
AsyncQ Server（并发消费）
  |
  v
消息处理器（Router）
  |-- notify:email     -> EmailAdapter
  |-- notify:sms       -> SMSAdapter
  |-- notify:wechat_work -> WeChatWorkAdapter
  |-- notify:dingtalk  -> DingTalkAdapter
  |-- notify:slack     -> SlackAdapter
  |
  v
渠道 SDK 发送
  |
  v
更新发送状态
```

### 重试机制

AsyncQ 内置了重试机制，支持三种延迟策略：

| 策略 | 行为 | 配置值 |
|------|------|--------|
| 指数退避 | 失败间隔指数增长（默认） | `"exponential"` |
| 线性增长 | 每次重试增加固定时间 | `"linear"` |
| 固定间隔 | 每次重试间隔相同 | `"fixed"` |

```toml
[component.asyncq]
maxRetry = 3
retryDelayFunc = "exponential"
```

### 队列优先级

通过队列优先级确保关键消息优先处理：

```toml
[component.asyncq.queues]
critical = 6    # 告警、验证码
default = 3     # 普通通知
low = 1         # 日报、统计
```

发送时指定队列：

```go
// 告警消息使用 critical 队列
s.asyncq.Enqueue(ctx, task, asynq.Queue("critical"))

// 日报使用 low 队列
s.asyncq.Enqueue(ctx, task, asynq.Queue("low"))
```

### 速率限制

防止渠道 API 被滥用，建议在适配器层实现速率限制：

```go
type RateLimitedAdapter struct {
    inner   NotifyAdapter
    limiter *rate.Limiter
}

func (a *RateLimitedAdapter) Send(ctx context.Context, payload NotifyPayload) error {
    if err := a.limiter.Wait(ctx); err != nil {
        return fmt.Errorf("rate limit exceeded: %w", err)
    }
    return a.inner.Send(ctx, payload)
}

// SMS 适配器：每秒最多 10 条
smsAdapter := &RateLimitedAdapter{
    inner:   NewAliyunSMSAdapter(config),
    limiter: rate.NewLimiter(10, 20),
}
```

## 常见问题

### 邮件发送失败

```text
dial tcp smtp.example.com:465: connect: connection refused
```

检查项：

1. SMTP 服务器地址和端口是否正确
2. SSL/TLS 设置是否匹配（465 端口通常用 SSL，587 端口用 STARTTLS）
3. 用户名密码是否正确
4. 防火墙是否放行 SMTP 端口

### 短信发送失败

```text
isv.INVALID_PARAMETERS
```

常见原因：

1. 签名名称与控制台注册的不一致
2. 模板变量缺少必填参数
3. 手机号格式不正确（需要带国际区号）
4. 短信服务账户余额不足

### 消息投递延迟

如果消息投递延迟明显：

1. 检查 AsyncQ Server 是否运行：`docker compose ps redis`
2. 增加并发数：

```toml
[component.asyncq]
concurrency = 50  # 默认 10
```

3. 检查 Redis 是否有慢查询：`redis-cli slowlog get 10`

### 企业微信 Webhook 发送失败

```text
{"errcode":93000,"errmsg":"invalid webhook url"}
```

检查项：

1. Webhook URL 是否完整（包含 key 参数）
2. 机器人是否已被移除或禁用
3. 消息体格式是否正确（Markdown / Text 格式）

### 重试耗尽后如何处理

当 `maxRetry` 次重试后仍然失败：

1. AsyncQ 会将任务移入 Dead 队列
2. 可以通过 Asynq CLI 查看：`asynq dash`
3. 建议在处理器中记录失败日志，并在数据库中标记状态
4. 可通过死信处理器（Dead job handler）进行告警或人工介入

## 参考链接

- [asynq - Go 异步任务队列](https://github.com/hibiken/asynq)
- [阿里云短信服务](https://help.aliyun.com/product/44282.html)
- [腾讯云短信服务](https://cloud.tencent.com/product/sms)
- [企业微信 Webhook 机器人](https://developer.work.weixin.qq.com/document/path/91770)
- [钉钉自定义机器人](https://open.dingtalk.com/document/orgapp/robot-overview)
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- EgoAdmin 源码：`internal/component/asyncq/`
