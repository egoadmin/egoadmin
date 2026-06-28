package auditlog

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/elib/pkg/metadata"
	"github.com/egoadmin/elib/pkg/transport/grpc"
	"github.com/gotomicro/ego/core/elog"
)

// Loger 日志
type Loger interface {
	// Save 保存日志
	// @param fnName 功能名称
	// @param typ 操作类型,可填写登录,登出,新增,修改,删除
	// @param action 请求名称
	// @param req 参数信息,需为结构体指针.
	// @param remarks 备注信息.
	Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string)
}

// AccessLogDetail 访问日志信息.
type AccessLogDetail struct {
	Action     string // 请求名称
	URL        string // 请求url
	OriginIP   string // 来源ip
	GrpcMethod string // 请求grpc方法
	Params     string // 请求参数
	Username   string // 用户名
	UserID     string // 用户id
	DeptID     string // 用户部门id
	Remark     string // 备注
}

// SaveFunc 日志保存方法
type SaveFunc func(ctx context.Context, alog AccessLogDetail) error

// Option is log option.
type Option func(*options)

// options is a log config
type options struct {
	sf SaveFunc
}

// Save 保存日志
// @param fnName 功能名称
// @param typ 操作类型,可填写登录,登出,新增,修改,删除
// @param action 请求名称
// @param req 参数信息,需为结构体指针.
// @param remarks 备注信息.
func (o *options) Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string) {
	var (
		err      error
		detail   AccessLogDetail
		reqBytes []byte
	)
	ctx = context.WithoutCancel(ctx)

	defer func() {
		if err != nil {
			elog.Error("日志记录失败", elog.FieldValueAny(detail))
		}
	}()

	if req != nil {
		if reqBytes, err = json.Marshal(req); err != nil {
			return
		}
		reqBytes = maskJSONSecrets(reqBytes)
	}

	actionArr := make([]string, 0)
	actionArr = append(actionArr, fnName, typ, action)
	auth, ok := authsession.FromContext(ctx)
	if ok {
		detail.Username = auth.Username
		detail.UserID = auth.UserIDString()
		detail.DeptID = auth.DeptIDString()
	}

	md := metadata.ExtractIncoming(ctx)
	detail.URL = md.Get(urlHeader)
	detail.OriginIP = md.Get(xForwardedForHeader)
	detail.GrpcMethod = grpc.FromContext(ctx)
	detail.Action = strings.Join(actionArr, ".")
	detail.Params = string(reqBytes)
	if len(remarks) > 0 {
		detail.Remark = strings.Join(remarks, ",")
	}

	go func(ctx context.Context, detail AccessLogDetail) {
		if err := o.sf(ctx, detail); err != nil {
			elog.Error("日志记录失败", elog.FieldErr(err), elog.FieldValueAny(detail))
		}
	}(ctx, detail)

	return
}

func maskJSONSecrets(raw []byte) []byte {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	masked := maskJSONValue(value)
	out, err := json.Marshal(masked)
	if err != nil {
		return raw
	}
	return out
}

func maskJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if isSecretLogKey(key) {
				v[key] = "***"
				continue
			}
			v[key] = maskJSONValue(item)
		}
	case []any:
		for i := range v {
			v[i] = maskJSONValue(v[i])
		}
	}
	return value
}

func isSecretLogKey(key string) bool {
	switch strings.ToLower(key) {
	case "password", "oldpassword", "old_password", "newpassword", "new_password",
		"passwordcipher", "password_cipher", "privatekey", "private_key", "privatekeypem", "private_key_pem",
		"keyid", "key_id", "challengeid", "challenge_id", "nonce", "captchacode", "captcha_code",
		"token", "refreshtoken", "refresh_token", "authorization":
		return true
	default:
		return false
	}
}

// New 新建日志
func New(fn SaveFunc, opts ...Option) Loger {
	o := &options{
		sf: fn,
	}
	for _, opt := range opts {
		opt(o)
	}

	return o
}
