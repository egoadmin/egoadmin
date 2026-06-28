package asyncq

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// AsyncqInterface asyncq接口定义
type AsyncqInterface interface {
	// 客户端方法
	ClientInterface

	// 服务端方法
	ServerInterface

	// 通用方法
	Close() error
	Health(ctx context.Context) error
}

// ClientInterface 客户端接口
type ClientInterface interface {
	// Enqueue 入队任务
	Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)

	// EnqueueIn 延迟入队任务
	EnqueueIn(ctx context.Context, delay time.Duration, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)

	// EnqueueAt 定时入队任务
	EnqueueAt(ctx context.Context, processAt time.Time, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// ServerInterface 服务端接口
type ServerInterface interface {
	// RegisterHandler 注册任务处理器
	RegisterHandler(pattern string, handler asynq.Handler)

	// RegisterHandlerFunc 注册任务处理函数
	RegisterHandlerFunc(pattern string, handler func(context.Context, *asynq.Task) error)

	// Start 启动服务端
	Start() error

	// Stop 停止服务端
	Stop() error

	// Shutdown 优雅关闭服务端
	Shutdown() error
}

// TaskHandler 任务处理器函数类型
type TaskHandler func(ctx context.Context, task *asynq.Task) error

// TaskInfo 任务信息
type TaskInfo struct {
	ID       string
	Type     string
	Payload  []byte
	Queue    string
	MaxRetry int
	Retried  int
	State    string
}
