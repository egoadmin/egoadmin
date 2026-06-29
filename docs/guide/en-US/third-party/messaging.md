# Messaging & Notifications

EgoAdmin provides multi-channel notification capabilities, supporting unified delivery across email, SMS, WeChat Work, DingTalk, and Slack.

## Overview

The messaging system uses a unified interface plus channel adapter architecture. Business code sends messages through a unified notification interface, while the underlying layer routes to the appropriate adapter based on channel type. Asynchronous delivery is handled by the AsyncQ component for non-blocking operation.

Architecture layers:

| Layer | Responsibility | Location |
|-------|---------------|----------|
| Interface | Unified notification interface, message structure definition | `internal/component/notify/` |
| Adapter | Channel-specific sending adapters | `internal/platform/notify/` |
| Async | Message queue delivery with retry | `internal/component/asyncq/` |

Supported messaging channels:

| Channel | Protocol | Use Case |
|---------|----------|----------|
| Email | SMTP | Registration confirmation, password reset, report notifications |
| SMS (Alibaba Cloud) | HTTP API | Verification codes, order status changes |
| SMS (Tencent Cloud) | HTTP API | Verification codes, marketing notifications |
| WeChat Work | Webhook | Internal alerts, approval notifications |
| DingTalk | Webhook | Internal alerts, work order notifications |
| Slack | Webhook | Dev team notifications, CI/CD alerts |

::: tip Design Principle
Messaging is asynchronous and best-effort. While 100% delivery is not guaranteed across all channels, retry mechanisms and status tracking maximize delivery success rates.
:::

## Core Usage

### AsyncQ Message Queue

AsyncQ is the asynchronous delivery foundation for messaging, built on [asynq](https://github.com/hibiken/asynq) with Redis as the message store:

```go
// internal/component/asyncq/interface.go

type AsyncqInterface interface {
    // Client methods
    ClientInterface

    // Server methods
    ServerInterface

    // Common methods
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

Wire injection:

```go
// internal/component/asyncq/provider.go
var ProviderSet = wire.NewSet(
    NewAsyncqComponent,
)
```

### Sending Messages

Business code enqueues message tasks:

```go
type NotifyPayload struct {
    Channel  string            `json:"channel"`  // email, sms, wechat_work, dingtalk, slack
    To       string            `json:"to"`        // Recipient
    Subject  string            `json:"subject"`   // Subject (email)
    Body     string            `json:"body"`      // Body text
    Template string            `json:"template"`  // Template name
    Vars     map[string]string `json:"vars"`      // Template variables
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

### Registering Message Handlers

Register handler functions for each channel on the server side:

```go
func (s *Server) initNotifyHandlers() {
    // Email notifications
    s.asyncq.RegisterHandlerFunc("notify:email", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.emailAdapter.Send(ctx, payload)
    })

    // SMS notifications
    s.asyncq.RegisterHandlerFunc("notify:sms", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.smsAdapter.Send(ctx, payload)
    })

    // WeChat Work bot
    s.asyncq.RegisterHandlerFunc("notify:wechat_work", func(ctx context.Context, task *asynq.Task) error {
        var payload NotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }
        return s.wechatWorkAdapter.Send(ctx, payload)
    })
}
```

### Delayed Delivery

Use AsyncQ's `EnqueueIn` for delayed delivery:

```go
// Send reminder in 5 minutes
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

### Scheduled Delivery

Use `EnqueueAt` to send at a specific time:

```go
// Send daily report reminder tomorrow at 9 AM
tomorrow9AM := time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
_, err := s.asyncq.EnqueueAt(ctx, tomorrow9AM, asynq.NewTask("notify:email", payload))
```

## Configuration Examples

### AsyncQ Component

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

# Queue priorities (higher number = higher priority)
[component.asyncq.queues]
critical = 6
default = 3
low = 1
```

### Email (SMTP)

```toml
[client.smtp]
host = "smtp.example.com"
port = 465
username = "noreply@example.com"
password = "smtp-password"
from = "EgoAdmin <noreply@example.com>"
ssl = true
```

### SMS (Alibaba Cloud)

```toml
[client.sms.aliyun]
accessKeyId = "your-access-key-id"
accessKeySecret = "your-access-key-secret"
signName = "EgoAdmin"
templateCode = "SMS_123456789"
```

### SMS (Tencent Cloud)

```toml
[client.sms.tencent]
secretId = "your-secret-id"
secretKey = "your-secret-key"
appId = "1400000000"
signName = "EgoAdmin"
templateId = "123456"
```

### WeChat Work Bot

```toml
[client.wechat_work]
webhookUrl = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=your-key"
```

### DingTalk Bot

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

### Environment Variable Overrides

```bash
# AsyncQ
EGOADMIN_COMPONENT_ASYNCQ_REDISADDR=redis.prod.example.com:6380
EGOADMIN_COMPONENT_ASYNCQ_REDISPASSWORD=prod-redis-password

# SMTP
EGOADMIN_CLIENT_SMTP_HOST=smtp.prod.example.com
EGOADMIN_CLIENT_SMTP_PASSWORD=prod-smtp-password

# WeChat Work
EGOADMIN_CLIENT_WECHAT_WORK_WEBHOOKURL=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=prod-key
```

## Real-World Examples

### Registration Verification Code

```go
func (s *UserService) SendRegisterCode(ctx context.Context, phone string) error {
    code := generateCode()

    // Store verification code in Redis (5-minute TTL)
    s.redis.Set(ctx, "register:code:"+phone, code, 5*time.Minute)

    // Send SMS asynchronously
    payload, _ := json.Marshal(NotifyPayload{
        Channel:  "sms",
        To:       phone,
        Template: "register_code",
        Vars:     map[string]string{"code": code},
    })
    _, err := s.asyncq.Enqueue(ctx, asynq.NewTask("notify:sms", payload))
    return err
}
```

### Multi-Channel Alerting

When the system detects an anomaly, notify multiple channels simultaneously:

```go
func (s *MonitorService) Alert(ctx context.Context, alert *Alert) error {
    // Construct message
    subject := fmt.Sprintf("[ALERT] %s - %s", alert.Level, alert.Title)
    body := fmt.Sprintf("Service: %s\nDetail: %s\nTime: %s",
        alert.Service, alert.Detail, alert.Time.Format(time.RFC3339))

    // Send to multiple channels in parallel
    channels := []string{"email", "wechat_work", "dingtalk"}
    for _, ch := range channels {
        payload, _ := json.Marshal(NotifyPayload{
            Channel: ch,
            To:      getChannelRecipient(ch, alert),
            Subject: subject,
            Body:    body,
        })
        // Use critical queue for high-priority alerts
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

### Template Messages

Use a template system to manage message formats uniformly:

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

// Use template in handler
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

### Status Tracking

Record message send status in MySQL for retry management and auditing:

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

// Record before sending
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

// Update after sending
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

## How It Works

### Message Delivery Flow

```text
Business code
  |
  v
asyncq.Enqueue(task)
  |
  v
Redis Queue (partitioned by priority: critical > default > low)
  |
  v
AsyncQ Server (concurrent consumption)
  |
  v
Message Handler (Router)
  |-- notify:email       -> EmailAdapter
  |-- notify:sms         -> SMSAdapter
  |-- notify:wechat_work -> WeChatWorkAdapter
  |-- notify:dingtalk    -> DingTalkAdapter
  |-- notify:slack       -> SlackAdapter
  |
  v
Channel SDK delivery
  |
  v
Update send status
```

### Retry Mechanism

AsyncQ has built-in retry with three delay strategies:

| Strategy | Behavior | Config Value |
|----------|----------|-------------|
| Exponential backoff | Failure interval grows exponentially (default) | `"exponential"` |
| Linear increase | Each retry adds a fixed duration | `"linear"` |
| Fixed interval | Same interval between retries | `"fixed"` |

```toml
[component.asyncq]
maxRetry = 3
retryDelayFunc = "exponential"
```

### Queue Priorities

Ensure critical messages are processed first via queue priorities:

```toml
[component.asyncq.queues]
critical = 6    # Alerts, verification codes
default = 3     # Normal notifications
low = 1         # Daily reports, statistics
```

Specify queue when sending:

```go
// Alerts use critical queue
s.asyncq.Enqueue(ctx, task, asynq.Queue("critical"))

// Daily reports use low queue
s.asyncq.Enqueue(ctx, task, asynq.Queue("low"))
```

### Rate Limiting

Prevent channel API abuse by implementing rate limiting at the adapter layer:

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

// SMS adapter: max 10 per second
smsAdapter := &RateLimitedAdapter{
    inner:   NewAliyunSMSAdapter(config),
    limiter: rate.NewLimiter(10, 20),
}
```

## Common Issues

### Email Send Failure

```text
dial tcp smtp.example.com:465: connect: connection refused
```

Check these items:

1. Is the SMTP server address and port correct?
2. Does the SSL/TLS setting match? (Port 465 typically uses SSL, port 587 uses STARTTLS)
3. Is the username/password correct?
4. Is the SMTP port open in the firewall?

### SMS Send Failure

```text
isv.INVALID_PARAMETERS
```

Common causes:

1. Signature name does not match the one registered in the console
2. Template variables are missing required parameters
3. Phone number format is incorrect (needs international country code)
4. SMS service account balance is insufficient

### Message Delivery Delay

If message delivery has noticeable delays:

1. Check if AsyncQ Server is running: `docker compose ps redis`
2. Increase concurrency:

```toml
[component.asyncq]
concurrency = 50  # Default: 10
```

3. Check Redis for slow queries: `redis-cli slowlog get 10`

### WeChat Work Webhook Send Failure

```text
{"errcode":93000,"errmsg":"invalid webhook url"}
```

Check these items:

1. Is the Webhook URL complete (including the key parameter)?
2. Has the bot been removed or disabled?
3. Is the message body format correct (Markdown / Text format)?

### What Happens When Retries Are Exhausted

When all `maxRetry` attempts fail:

1. AsyncQ moves the task to the Dead queue
2. View dead jobs via the Asynq CLI: `asynq dash`
3. Record failure logs in the handler and mark status in the database
4. Use a Dead job handler for alerting or manual intervention

## Reference Links

- [asynq - Go Async Task Queue](https://github.com/hibiken/asynq)
- [Alibaba Cloud SMS Service](https://www.alibabacloud.com/product/sms)
- [Tencent Cloud SMS Service](https://www.tencentcloud.com/product/sms)
- [WeChat Work Webhook Bot](https://developer.work.weixin.qq.com/document/path/91770)
- [DingTalk Custom Bot](https://open.dingtalk.com/document/orgapp/robot-overview)
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- EgoAdmin source: `internal/component/asyncq/`
