# Payment Integration

EgoAdmin provides payment component integration patterns and infrastructure, supporting payment channels like WeChat Pay and Alipay.

## Overview

EgoAdmin does not directly wrap specific payment SDKs. Instead, it provides a component-based integration pattern: create payment component wrappers in `internal/component/` or `internal/platform/`, use AsyncQ for asynchronous callback handling and status updates, and leverage DTM for cross-service payment transaction consistency.

Integration layers:

| Layer | Responsibility | Location |
|-------|---------------|----------|
| Component | Payment SDK wrapper, signature verification | `internal/component/payment/` |
| Application | Create orders, query status, refunds | `internal/app/*/application/` |
| Async | Callback processing, status notifications | `internal/component/asyncq/` |
| Transaction | Cross-service consistency | DTM Saga / TCC |

::: tip Project-Specific
Payment integration code is project-specific. EgoAdmin provides the component pattern and infrastructure (callback handling, AsyncQ async processing), but actual payment SDK integration depends on your business requirements. The examples below show recommended integration patterns.
:::

## Core Usage

### WeChat Pay

WeChat Pay supports multiple payment modes:

| Mode | Scenario | Description |
|------|----------|-------------|
| JSAPI | Public account / Mini Program | User opens page in WeChat, invokes WeChat Pay |
| Native | PC website QR payment | Generate payment QR code, user scans to pay |
| H5 | Mobile browser payment | Redirects to WeChat client to complete payment |
| Mini Program | Mini Program payment | Invokes WeChat Pay within a Mini Program |

#### Integration Pattern

```go
// internal/component/payment/wechat/config.go
type WechatPayConfig struct {
    AppID           string `toml:"appId"`
    MchID           string `toml:"mchId"`
    APIv3Key        string `toml:"apiv3Key"`
    CertPath        string `toml:"certPath"`
    KeyPath         string `toml:"keyPath"`
    NotifyURL       string `toml:"notifyURL"`
    RefundNotifyURL string `toml:"refundNotifyURL"`
}

// internal/component/payment/wechat/component.go
type WechatPayComponent struct {
    config *WechatPayConfig
    client *wechat.Client
    logger *elog.Component
}

// Create JSAPI payment parameters
func (c *WechatPayComponent) CreateJSAPIPayment(ctx context.Context, order *PaymentOrder) (*JSAPIParams, error) {
    // Call WeChat Pay unified order API
    prepayID, err := c.client.Prepay(ctx, &wechat.PrepayRequest{
        AppID:       c.config.AppID,
        MchID:       c.config.MchID,
        Description: order.Description,
        OutTradeNo:  order.OrderNo,
        Amount: &wechat.Amount{
            Total:    order.Amount,
            Currency: "CNY",
        },
        Payer: &wechat.Payer{
            OpenID: order.OpenID,
        },
        NotifyURL: c.config.NotifyURL,
    })
    if err != nil {
        return nil, fmt.Errorf("wechat prepay failed: %w", err)
    }

    // Generate parameters for frontend to invoke payment
    return &JSAPIParams{
        PrepayID: prepayID.PrepayID,
        // Frontend uses these params to call wx.chooseWXPay
    }, nil
}
```

#### Callback Handling

```go
func (c *WechatPayComponent) HandleNotify(ctx context.Context, body io.Reader) (*NotifyResult, error) {
    // 1. Verify signature
    notification, err := c.client.ParseNotification(ctx, body)
    if err != nil {
        return nil, fmt.Errorf("parse notification failed: %w", err)
    }

    // 2. Decrypt notification data
    result, err := c.client.DecryptNotification(ctx, notification)
    if err != nil {
        return nil, fmt.Errorf("decrypt notification failed: %w", err)
    }

    // 3. Return result; business layer handles order status update
    return &NotifyResult{
        OrderNo:       result.OutTradeNo,
        TransactionID: result.TransactionID,
        Status:        result.TradeState,
        Amount:        result.Amount.Total,
    }, nil
}
```

### Alipay

Alipay supports the following payment modes:

| Mode | Scenario | Description |
|------|----------|-------------|
| Face-to-Face | Offline QR payment | Merchant displays QR code, user scans to pay |
| PC Website | PC web payment | Redirects to Alipay checkout page |
| Mobile Website | H5 payment | Redirects to Alipay APP to complete payment |
| APP Payment | In-APP payment | Invokes Alipay client within an APP |

#### Integration Pattern

```go
// internal/component/payment/alipay/config.go
type AlipayConfig struct {
    AppID            string `toml:"appId"`
    PrivateKey       string `toml:"privateKey"`
    AlipayPublicKey  string `toml:"alipayPublicKey"`
    NotifyURL        string `toml:"notifyURL"`
    ReturnURL        string `toml:"returnURL"`
    Sandbox          bool   `toml:"sandbox"`
}

// internal/component/payment/alipay/component.go
type AlipayComponent struct {
    config *AlipayConfig
    client *alipay.Client
    logger *elog.Component
}

// Create face-to-face payment
func (c *AlipayComponent) CreateFaceToFacePayment(ctx context.Context, order *PaymentOrder) (*QRCodeParams, error) {
    result, err := c.client.TradePrecreate(ctx, &alipay.TradePrecreateRequest{
        OutTradeNo:  order.OrderNo,
        TotalAmount: formatAmount(order.Amount),
        Subject:     order.Description,
    })
    if err != nil {
        return nil, fmt.Errorf("alipay precreate failed: %w", err)
    }

    return &QRCodeParams{
        QRCodeURL: result.QRCode,
    }, nil
}

// Create page payment
func (c *AlipayComponent) CreatePagePayment(ctx context.Context, order *PaymentOrder) (*PagePayParams, error) {
    result, err := c.client.TradePagePay(ctx, &alipay.TradePagePayRequest{
        OutTradeNo:  order.OrderNo,
        TotalAmount: formatAmount(order.Amount),
        Subject:     order.Description,
        ProductCode: "FAST_INSTANT_TRADE_PAY",
    })
    if err != nil {
        return nil, fmt.Errorf("alipay page pay failed: %w", err)
    }

    return &PagePayParams{
        PayURL: result.PayURL,
    }, nil
}
```

## Configuration Examples

### WeChat Pay

```toml
[component.payment.wechat]
appId = "wx1234567890abcdef"
mchId = "1238900001"
apiv3Key = "your-apiv3-key"
certPath = "./certs/wechat/apiclient_cert.pem"
keyPath = "./certs/wechat/apiclient_key.pem"
notifyURL = "https://api.example.com/payment/wechat/notify"
refundNotifyURL = "https://api.example.com/payment/wechat/refund-notify"
```

### Alipay

```toml
[component.payment.alipay]
appId = "2021000000000001"
privateKey = "MIIEvgIBADANBgkqh..."
alipayPublicKey = "MIIBIjANBgkqh..."
notifyURL = "https://api.example.com/payment/alipay/notify"
returnURL = "https://example.com/payment/success"
sandbox = false
```

### AsyncQ Callback Processing

```toml
[component.asyncq]
redisAddr = "127.0.0.1:6380"
enableClient = true
enableServer = true
concurrency = 20
maxRetry = 5
retryDelayFunc = "exponential"
taskTimeout = "30s"
```

### Environment Variable Overrides

```bash
# WeChat Pay
EGOADMIN_COMPONENT_PAYMENT_WECHAT_APPID=wx1234567890abcdef
EGOADMIN_COMPONENT_PAYMENT_WECHAT_MCHID=1238900001
EGOADMIN_COMPONENT_PAYMENT_WECHAT_APIV3KEY=prod-apiv3-key
EGOADMIN_COMPONENT_PAYMENT_WECHAT_NOTIFYURL=https://api.example.com/payment/wechat/notify

# Alipay
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_APPID=2021000000000001
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_PRIVATEKEY=prod-private-key
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_ALIPAYPUBLICKEY=prod-alipay-public-key
```

::: warning
Private keys and API secrets must never be committed to the code repository. Use environment variables or external secret management services (like Vault) for injection.
:::

## Real-World Examples

### Complete Payment Flow

```go
type PaymentService struct {
    wechat    *WechatPayComponent
    alipay    *AlipayComponent
    asyncq    *asyncq.Component
    orderRepo OrderRepository
}

// Create payment order
func (s *PaymentService) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*PaymentResponse, error) {
    // 1. Generate order number
    orderNo := generateOrderNo()

    // 2. Create order record (status: pending)
    order := &Order{
        OrderNo:     orderNo,
        Amount:      req.Amount,
        Description: req.Description,
        Status:      "pending",
        Channel:     req.Channel,
    }
    if err := s.orderRepo.Create(ctx, order); err != nil {
        return nil, err
    }

    // 3. Create payment parameters by channel
    switch req.Channel {
    case "wechat_jsapi":
        params, err := s.wechat.CreateJSAPIPayment(ctx, &PaymentOrder{
            OrderNo:     orderNo,
            Amount:      req.Amount,
            Description: req.Description,
            OpenID:      req.OpenID,
        })
        if err != nil {
            return nil, err
        }
        return &PaymentResponse{OrderNo: orderNo, Params: params}, nil

    case "alipay_page":
        params, err := s.alipay.CreatePagePayment(ctx, &PaymentOrder{
            OrderNo:     orderNo,
            Amount:      req.Amount,
            Description: req.Description,
        })
        if err != nil {
            return nil, err
        }
        return &PaymentResponse{OrderNo: orderNo, Params: params}, nil

    default:
        return nil, fmt.Errorf("unsupported payment channel: %s", req.Channel)
    }
}
```

### Callback Notification Handling

```go
// HTTP callback endpoint
func (s *PaymentController) HandleWechatNotify(c *gin.Context) {
    result, err := s.wechat.HandleNotify(c.Request.Context(), c.Request.Body)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": err.Error()})
        return
    }

    // Process order status update asynchronously (avoid callback timeout)
    payload, _ := json.Marshal(PaymentNotifyPayload{
        OrderNo:       result.OrderNo,
        TransactionID: result.TransactionID,
        Status:        result.Status,
        Amount:        result.Amount,
        Channel:       "wechat",
    })
    s.asyncq.Enqueue(c.Request.Context(), asynq.NewTask("payment:confirm", payload))

    // Return success to WeChat
    c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}

// Async order confirmation
func (s *PaymentService) initPaymentConfirmHandler() {
    s.asyncq.RegisterHandlerFunc("payment:confirm", func(ctx context.Context, task *asynq.Task) error {
        var payload PaymentNotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }

        // Query order
        order, err := s.orderRepo.GetByOrderNo(ctx, payload.OrderNo)
        if err != nil {
            return err
        }

        // Idempotency check: skip already processed orders
        if order.Status != "pending" {
            return nil
        }

        // Verify amount
        if order.Amount != payload.Amount {
            slog.Error("payment amount mismatch",
                "order", payload.OrderNo,
                "expected", order.Amount,
                "actual", payload.Amount)
            return fmt.Errorf("amount mismatch")
        }

        // Update order status
        return s.orderRepo.UpdateStatus(ctx, payload.OrderNo, "paid", &payload.TransactionID)
    })
}
```

### Cross-Service Payment (DTM Saga)

When payment involves multiple services (e.g., deduct stock + update order + record accounting), use DTM Saga for consistency:

```go
func (s *PaymentService) ProcessPaymentSuccess(ctx context.Context, order *Order) error {
    gid := dtmgrpc.MustGenGid(s.dtmServer)

    saga := dtmgrpc.NewSagaGrpc(s.dtmServer, gid).
        Add(
            // Deduct stock
            s.stockTarget+"/stock.v1.StockDtmService/DeductStock",
            s.stockTarget+"/stock.v1.StockDtmService/RevertStock",
            &DeductStockRequest{OrderNo: order.OrderNo, Items: order.Items},
        ).
        Add(
            // Record accounting
            s.accountTarget+"/account.v1.AccountDtmService/RecordIncome",
            s.accountTarget+"/account.v1.AccountDtmService/RevertIncome",
            &RecordIncomeRequest{OrderNo: order.OrderNo, Amount: order.Amount},
        )

    return saga.Submit()
}
```

### Query and Refund

```go
// Query payment status
func (s *PaymentService) QueryPaymentStatus(ctx context.Context, orderNo string) (string, error) {
    order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
    if err != nil {
        return "", err
    }

    // If local status is pending, actively query the third party
    if order.Status == "pending" {
        switch order.Channel {
        case "wechat_jsapi", "wechat_native":
            result, err := s.wechat.QueryOrder(ctx, orderNo)
            if err != nil {
                return "", err
            }
            if result.TradeState == "SUCCESS" {
                s.orderRepo.UpdateStatus(ctx, orderNo, "paid", &result.TransactionID)
                return "paid", nil
            }
        case "alipay_page", "alipay_face":
            result, err := s.alipay.QueryOrder(ctx, orderNo)
            if err != nil {
                return "", err
            }
            if result.TradeStatus == "TRADE_SUCCESS" {
                s.orderRepo.UpdateStatus(ctx, orderNo, "paid", &result.TradeNo)
                return "paid", nil
            }
        }
    }

    return order.Status, nil
}

// Refund
func (s *PaymentService) Refund(ctx context.Context, req *RefundRequest) error {
    order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
    if err != nil {
        return err
    }

    switch order.Channel {
    case "wechat_jsapi", "wechat_native":
        return s.wechat.Refund(ctx, &WechatRefundRequest{
            OrderNo:       req.OrderNo,
            RefundNo:      req.RefundNo,
            TotalAmount:   order.Amount,
            RefundAmount:  req.Amount,
        })
    case "alipay_page", "alipay_face":
        return s.alipay.Refund(ctx, &AlipayRefundRequest{
            OrderNo:      req.OrderNo,
            RefundNo:     req.RefundNo,
            RefundAmount: req.Amount,
        })
    }
    return fmt.Errorf("unsupported channel: %s", order.Channel)
}
```

## How It Works

### Payment Flow Sequence

```text
Frontend            Backend                 WeChat/Alipay
  |                   |                       |
  |-- Create order -->|                       |
  |                   |-- Unified order ----->|
  |                   |<-- Prepay ID ---------|
  |<-- Pay params ----|                       |
  |                   |                       |
  |-- Invoke pay ---->|--------------------->|
  |                   |                       |
  |                   |<-- Async callback ----|
  |                   |-- Verify + Decrypt    |
  |                   |-- AsyncQ enqueue      |
  |                   |                       |
  |                   |-- Consumer processes  |
  |                   |   Idempotency check   |
  |                   |   Amount verification |
  |                   |   Update order status |
  |                   |                       |
  |-- Query status -->|                       |
  |<-- paid ----------|                       |
```

### Security Essentials

```text
1. HTTPS Callbacks
   |-- Callback URLs must use HTTPS
   |-- Certificates verified by payment platform

2. Signature Verification
   |-- Every callback request must be signature-verified
   |-- WeChat: RSA-SHA256 v3 signature
   |-- Alipay: RSA2 signature

3. IP Whitelisting
   |-- WeChat Pay: configure notification IP whitelist
   |-- Alipay: configure gateway whitelist

4. Idempotent Processing
   |-- Callbacks may arrive multiple times (network retries)
   |-- Determine if already processed by order status
   |-- Use optimistic locking or unique index for DB operations

5. Amount Verification
   |-- Callback amount must match order amount
   |-- Prevents amount tampering attacks
```

### Callback Timeout and Retry

| Payment Platform | Timeout | Retry Strategy |
|-----------------|---------|---------------|
| WeChat Pay | 5s | 15s / 15s / 30s / 3m / 10m / 20m / 30m / 1h / 2h / 6h |
| Alipay | Within 25h | 2m / 10m / 10m / 1h / 2h / 6h (7 total attempts) |

::: warning Callback Processing Must Be Fast
The callback endpoint must return a success response within the timeout window. Complex business logic (notifying users, updating related data) should be processed asynchronously via AsyncQ to avoid callback timeouts that trigger duplicate notifications.
:::

## Common Issues

### Callback Not Received

```text
Payment successful but order status not updated
```

Troubleshooting steps:

1. Check if the callback URL uses HTTPS
2. Check if the server firewall allows traffic from payment platform IP ranges
3. Check if the callback URL is correctly configured (domain, path)
4. WeChat Pay: Log into the merchant platform and check notification configuration
5. Alipay: Log into the open platform and check async notification configuration
6. Check if the callback handler returned the correct success response (WeChat returns `SUCCESS`, Alipay returns `success`)

### Signature Verification Failed

```text
verify signature failed
```

Common causes:

1. API key or certificate mismatch
2. Using sandbox keys against the production environment
3. Alipay public key not updated (after certificate renewal)
4. WeChat APIv3 key configuration error

### Order Status Inconsistency

```text
Third party shows payment successful, but local order status is pending
```

Solution:

1. Implement a scheduled task to actively query orders pending for over 30 minutes
2. Use `QueryPaymentStatus` to synchronize status
3. Callback processing must be idempotent; duplicate callbacks should not cause data errors

### Refund Failure

```text
refund not allowed: order not fully paid
```

Common causes:

1. Refund amount exceeds order amount
2. Order payment not yet confirmed
3. Requesting refund after full refund already processed
4. Refund amount precision mismatch (WeChat/Alipay use fen, business layer may use yuan)

### Docker Compose Environment

Payment integration does not require local middleware services. Key dependencies:

1. HTTPS certificates (callbacks require HTTPS)
2. Correct DNS resolution (callback URL points to your server)
3. For local development, use ngrok / frp or similar tools to expose callback URLs

## Reference Links

- [WeChat Pay Developer Documentation](https://pay.weixin.qq.com/wiki/doc/apiv3/)
- [WeChat Pay Go SDK](https://github.com/wechatpay-apiv3/wechatpay-go)
- [Alipay Open Platform](https://open.alipay.com/)
- [Alipay Go SDK](https://github.com/smartwalle/alipay/v3)
- EgoAdmin source: `internal/component/asyncq/`
- EgoAdmin source: `internal/component/` (create your own payment component)
