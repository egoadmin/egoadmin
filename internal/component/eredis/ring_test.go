package eredis

import (
	"testing"

	"github.com/gotomicro/ego/core/elog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestRingMode(t *testing.T) {
	// 测试 Ring 模式的基本功能
	container := DefaultContainer()
	container.logger = testLogger()
	container.config.Mode = RingMode
	container.config.Addrs = []string{"localhost:6379", "localhost:6380"}
	container.config.Password = ""
	container.config.OnFail = "error" // 设置为 error 而不是 panic，避免测试时连接失败

	component := container.Build()
	assert.NotNil(t, component)

	// 测试获取 Ring 客户端
	ringClient := component.Ring()
	assert.NotNil(t, ringClient)

	// 测试模式识别
	assert.Equal(t, RingMode, component.Mode())

	// 测试通用客户端接口
	cmdable := component.Client()
	assert.NotNil(t, cmdable)

	// 测试锁客户端
	lockClient := component.LockClient()
	assert.NotNil(t, lockClient)
}

func TestWithRingOption(t *testing.T) {
	// 测试 WithRing 选项函数
	container := DefaultContainer()
	WithRing()(container)
	assert.Equal(t, RingMode, container.config.Mode)
}

func TestRingConfigValidation(t *testing.T) {
	// 测试配置验证
	container := DefaultContainer()
	container.logger = testLogger()
	container.config.Mode = RingMode
	container.config.Addrs = []string{} // 空地址列表

	// 应该 panic
	assert.Panics(t, func() {
		container.Build()
	})
}

func testLogger() *elog.Component {
	return elog.DefaultContainer().Build(elog.WithZapCore(zapcore.NewNopCore()))
}
