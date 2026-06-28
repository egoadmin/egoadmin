package discovery

import (
	"context"
	"errors"

	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/google/wire"
	"github.com/gotomicro/ego-component/eetcd"
	"github.com/gotomicro/ego-component/eetcd/registry"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/core/eregistry"
)

var ProviderSet = wire.NewSet(NewRegistry, NewReady)

type Ready struct{}

func NewReady(reg eregistry.Registry) Ready {
	return Ready{}
}

// NewRegistry 初始化服务注册/发现组件。
// 依赖 config.EgoReady 以保证在 ego 配置加载（econf 就绪）之后构造。
func NewRegistry(ready config.EgoReady) eregistry.Registry {
	if !hasConfig("etcd") || !hasConfig("registry") {
		return eregistry.Nop{}
	}
	return registry.Load("registry").Build(
		registry.WithClientEtcd(eetcd.Load("etcd").Build()),
	)
}

func EnsureResolver(ready config.EgoReady) Ready {
	return NewReady(NewRegistry(ready))
}

func hasConfig(key string) bool {
	var raw map[string]any
	err := econf.UnmarshalKey(key, &raw)
	if err == nil {
		return true
	}
	if errors.Is(err, econf.ErrInvalidKey) {
		return false
	}
	elog.Panic("parse discovery config error", elog.FieldErr(err), elog.FieldKey(key))
	return false
}

func Close(ctx context.Context, reg eregistry.Registry) error {
	done := make(chan error, 1)
	go func() {
		done <- reg.Close()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
