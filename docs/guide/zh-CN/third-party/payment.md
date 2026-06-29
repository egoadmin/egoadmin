# 支付集成

EgoAdmin 提供支付组件的集成模式和基础设施，支持微信支付和支付宝等支付渠道的接入。

## 概述

EgoAdmin 不直接封装具体的支付 SDK，而是提供一套组件化集成模式：在 `internal/component/` 或 `internal/platform/` 中创建支付组件包装器，利用 AsyncQ 处理异步回调和状态更新，通过 DTM 保障跨服务支付事务的一致性。

集成层次：

| 层次 | 职责 | 位置 |
|------|------|------|
| 组件层 | 支付 SDK 封装、签名验签 | `internal/component/payment/` |
| 应用层 | 创建订单、查询状态、退款 | `internal/app/*/application/` |
| 异步层 | 回调处理、状态通知 | `internal/component/asyncq/` |
| 事务层 | 跨服务一致性 | DTM Saga / TCC |

::: tip 项目特定
支付集成代码是项目特定的。EgoAdmin 提供组件模式和基础设施（回调处理、AsyncQ 异步处理），但实际的支付 SDK 集成取决于你的业务需求。以下示例展示的是推荐的集成模式。
:::

## 核心用法

### 微信支付

微信支付支持多种支付模式：

| 模式 | 场景 | 说明 |
|------|------|------|
| JSAPI | 公众号/小程序内支付 | 用户在微信内打开页面，调起微信支付 |
| Native | PC 网站扫码支付 | 生成支付二维码，用户扫码支付 |
| H5 | 手机浏览器支付 | 跳转微信客户端完成支付 |
| 小程序 | 小程序支付 | 小程序内调起微信支付 |

#### 集成模式

```go
// internal/component/payment/wechat/config.go
type WechatPayConfig struct {
    AppID        string `toml:"appId"`
    MchID        string `toml:"mchId"`
    APIv3Key     string `toml:"apiv3Key"`
    CertPath     string `toml:"certPath"`
    KeyPath      string `toml:"keyPath"`
    NotifyURL    string `toml:"notifyURL"`
    RefundNotifyURL string `toml:"refundNotifyURL"`
}

// internal/component/payment/wechat/component.go
type WechatPayComponent struct {
    config *WechatPayConfig
    client *wechat.Client
    logger *elog.Component
}

// 创建 JSAPI 支付参数
func (c *WechatPayComponent) CreateJSAPIPayment(ctx context.Context, order *PaymentOrder) (*JSAPIParams, error) {
    // 调用微信支付统一下单 API
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

    // 生成前端调起支付的参数
    return &JSAPIParams{
        PrepayID: prepayID.PrepayID,
        // 前端使用这些参数调用 wx.chooseWXPay
    }, nil
}
```

#### 回调处理

```go
func (c *WechatPayComponent) HandleNotify(ctx context.Context, body io.Reader) (*NotifyResult, error) {
    // 1. 验证签名
    notification, err := c.client.ParseNotification(ctx, body)
    if err != nil {
        return nil, fmt.Errorf("parse notification failed: %w", err)
    }

    // 2. 解密通知数据
    result, err := c.client.DecryptNotification(ctx, notification)
    if err != nil {
        return nil, fmt.Errorf("decrypt notification failed: %w", err)
    }

    // 3. 返回结果，业务层处理订单状态更新
    return &NotifyResult{
        OrderNo:     result.OutTradeNo,
        TransactionID: result.TransactionID,
        Status:      result.TradeState,
        Amount:      result.Amount.Total,
    }, nil
}
```

### 支付宝

支付宝支持以下支付模式：

| 模式 | 场景 | 说明 |
|------|------|------|
| 当面付 | 线下扫码 | 商户展示二维码，用户扫码付款 |
| 电脑网站 | PC 网页支付 | 跳转支付宝收银台页面 |
| 手机网站 | H5 支付 | 跳转支付宝 APP 完成支付 |
| APP 支付 | APP 内支付 | APP 内调起支付宝客户端 |

#### 集成模式

```go
// internal/component/payment/alipay/config.go
type AlipayConfig struct {
    AppID        string `toml:"appId"`
    PrivateKey   string `toml:"privateKey"`
    AlipayPublicKey string `toml:"alipayPublicKey"`
    NotifyURL    string `toml:"notifyURL"`
    ReturnURL    string `toml:"returnURL"`
    Sandbox      bool   `toml:"sandbox"`
}

// internal/component/payment/alipay/component.go
type AlipayComponent struct {
    config *AlipayConfig
    client *alipay.Client
    logger *elog.Component
}

// 创建当面付支付
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

// 创建网页支付
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

## 配置示例

### 微信支付

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

### 支付宝

```toml
[component.payment.alipay]
appId = "2021000000000001"
privateKey = "MIIEvgIBADANBgkqh..."
alipayPublicKey = "MIIBIjANBgkqh..."
notifyURL = "https://api.example.com/payment/alipay/notify"
returnURL = "https://example.com/payment/success"
sandbox = false
```

### AsyncQ 回调处理

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

### 环境变量覆盖

```bash
# 微信支付
EGOADMIN_COMPONENT_PAYMENT_WECHAT_APPID=wx1234567890abcdef
EGOADMIN_COMPONENT_PAYMENT_WECHAT_MCHID=1238900001
EGOADMIN_COMPONENT_PAYMENT_WECHAT_APIV3KEY=prod-apiv3-key
EGOADMIN_COMPONENT_PAYMENT_WECHAT_NOTIFYURL=https://api.example.com/payment/wechat/notify

# 支付宝
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_APPID=2021000000000001
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_PRIVATEKEY=prod-private-key
EGOADMIN_COMPONENT_PAYMENT_ALIPAY_ALIPAYPUBLICKEY=prod-alipay-public-key
```

::: warning
私钥和 API 密钥等敏感配置绝不能提交到代码仓库。使用环境变量或外部密钥管理服务（如 Vault）注入。
:::

## 实战示例

### 完整支付流程

```go
type PaymentService struct {
    wechat   *WechatPayComponent
    alipay   *AlipayComponent
    asyncq   *asyncq.Component
    orderRepo OrderRepository
}

// 创建支付订单
func (s *PaymentService) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*PaymentResponse, error) {
    // 1. 生成订单号
    orderNo := generateOrderNo()

    // 2. 创建订单记录（状态：待支付）
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

    // 3. 根据渠道创建支付参数
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

### 回调通知处理

```go
// HTTP 回调端点
func (s *PaymentController) HandleWechatNotify(c *gin.Context) {
    result, err := s.wechat.HandleNotify(c.Request.Context(), c.Request.Body)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": err.Error()})
        return
    }

    // 异步处理订单状态更新（避免回调超时）
    payload, _ := json.Marshal(PaymentNotifyPayload{
        OrderNo:       result.OrderNo,
        TransactionID: result.TransactionID,
        Status:        result.Status,
        Amount:        result.Amount,
        Channel:       "wechat",
    })
    s.asyncq.Enqueue(c.Request.Context(), asynq.NewTask("payment:confirm", payload))

    // 返回成功给微信
    c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}

// 异步确认订单
func (s *PaymentService) initPaymentConfirmHandler() {
    s.asyncq.RegisterHandlerFunc("payment:confirm", func(ctx context.Context, task *asynq.Task) error {
        var payload PaymentNotifyPayload
        if err := json.Unmarshal(task.Payload(), &payload); err != nil {
            return err
        }

        // 查询订单
        order, err := s.orderRepo.GetByOrderNo(ctx, payload.OrderNo)
        if err != nil {
            return err
        }

        // 幂等检查：已处理的订单不再处理
        if order.Status != "pending" {
            return nil
        }

        // 验证金额
        if order.Amount != payload.Amount {
            slog.Error("payment amount mismatch",
                "order", payload.OrderNo,
                "expected", order.Amount,
                "actual", payload.Amount)
            return fmt.Errorf("amount mismatch")
        }

        // 更新订单状态
        return s.orderRepo.UpdateStatus(ctx, payload.OrderNo, "paid", &payload.TransactionID)
    })
}
```

### 跨服务支付（DTM Saga）

当支付涉及多个服务时（如扣减库存 + 更新订单 + 记录账务），使用 DTM Saga 保证一致性：

```go
func (s *PaymentService) ProcessPaymentSuccess(ctx context.Context, order *Order) error {
    gid := dtmgrpc.MustGenGid(s.dtmServer)

    saga := dtmgrpc.NewSagaGrpc(s.dtmServer, gid).
        Add(
            // 扣减库存
            s.stockTarget+"/stock.v1.StockDtmService/DeductStock",
            s.stockTarget+"/stock.v1.StockDtmService/RevertStock",
            &DeductStockRequest{OrderNo: order.OrderNo, Items: order.Items},
        ).
        Add(
            // 记录账务
            s.accountTarget+"/account.v1.AccountDtmService/RecordIncome",
            s.accountTarget+"/account.v1.AccountDtmService/RevertIncome",
            &RecordIncomeRequest{OrderNo: order.OrderNo, Amount: order.Amount},
        )

    return saga.Submit()
}
```

### 查询和退款

```go
// 查询支付状态
func (s *PaymentService) QueryPaymentStatus(ctx context.Context, orderNo string) (string, error) {
    order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
    if err != nil {
        return "", err
    }

    // 如果本地状态是 pending，主动查询第三方
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

// 退款
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

## 工作原理

### 支付流程时序

```text
前端               后端                   微信/支付宝
  |                 |                       |
  |-- 创建订单 ---->|                       |
  |                 |-- 统一下单 ---------->|
  |                 |<-- 预支付ID -----------|
  |<-- 支付参数 ----|                       |
  |                 |                       |
  |-- 调起支付 ---->|--------------------->|
  |                 |                       |
  |                 |<-- 异步回调 ----------|
  |                 |-- 验签 + 解密         |
  |                 |-- AsyncQ 入队         |
  |                 |                       |
  |                 |-- 消费者处理          |
  |                 |   幂等检查            |
  |                 |   金额校验            |
  |                 |   更新订单状态        |
  |                 |                       |
  |-- 查询状态 ---->|                       |
  |<-- paid --------|                       |
```

### 安全要点

```text
1. HTTPS 回调
   |-- 回调 URL 必须使用 HTTPS
   |-- 证书由支付平台验证

2. 签名验证
   |-- 每个回调请求必须验签
   |-- 微信：RSA-SHA256 v3 签名
   |-- 支付宝：RSA2 签名

3. IP 白名单
   |-- 微信支付配置通知 IP 白名单
   |-- 支付宝配置网关白名单

4. 幂等处理
   |-- 回调可能重复到达（网络重试）
   |-- 通过订单状态判断是否已处理
   |-- 数据库操作加乐观锁或唯一索引

5. 金额校验
   |-- 回调中的金额必须与订单金额一致
   |-- 防止篡改金额攻击
```

### 回调超时与重试

| 支付平台 | 超时时间 | 重试策略 |
|---------|---------|---------|
| 微信支付 | 5 秒 | 15s / 15s / 30s / 3m / 10m / 20m / 30m / 1h / 2h / 6h |
| 支付宝 | 25 小时内 | 2m / 10m / 10m / 1h / 2h / 6h（共 7 次） |

::: warning 回调处理必须快速
回调接口必须在超时时间内返回成功响应。复杂业务逻辑（如通知用户、更新关联数据）应通过 AsyncQ 异步处理，避免回调超时导致重复通知。
:::

## 常见问题

### 回调未收到

```text
支付成功但订单状态未更新
```

排查步骤：

1. 检查回调 URL 是否使用 HTTPS
2. 检查服务器防火墙是否放行支付平台 IP 段
3. 检查回调 URL 是否正确配置（域名、路径）
4. 微信支付：登录商户平台检查「支付通知」配置
5. 支付宝：登录开放平台检查「异步通知」配置
6. 检查回调处理是否返回了正确的成功响应（微信返回 `SUCCESS`，支付宝返回 `success`）

### 签名验证失败

```text
verify signature failed
```

常见原因：

1. API 密钥或证书不匹配
2. 使用了沙箱密钥但请求了正式环境
3. 支付宝公钥未更新（证书换绑后）
4. 微信 APIv3 密钥配置错误

### 订单状态不一致

```text
第三方显示支付成功，但本地订单状态是 pending
```

解决：

1. 实现定时任务，主动查询 pending 超过 30 分钟的订单
2. 使用 `QueryPaymentStatus` 同步状态
3. 回调处理必须幂等，重复回调不会导致数据错误

### 退款失败

```text
refund not allowed: order not fully paid
```

常见原因：

1. 退款金额超过订单金额
2. 订单尚未完成支付确认
3. 已全额退款后再次申请
4. 退款金额精度不一致（微信/支付宝使用分，业务层可能使用元）

### Docker Compose 环境

支付集成不需要本地中间件服务。主要依赖：

1. HTTPS 证书（回调需要）
2. 正确的 DNS 解析（回调 URL 指向你的服务器）
3. 本地开发可使用 ngrok / frp 等内网穿透工具暴露回调 URL

## 参考链接

- [微信支付开发文档](https://pay.weixin.qq.com/wiki/doc/apiv3/)
- [微信支付 Go SDK](https://github.com/wechatpay-apiv3/wechatpay-go)
- [支付宝开放平台](https://open.alipay.com/)
- [支付宝 Go SDK](https://github.com/smartwalle/alipay/v3)
- EgoAdmin 源码：`internal/component/asyncq/`
- EgoAdmin 源码：`internal/component/`（创建你自己的支付组件）
