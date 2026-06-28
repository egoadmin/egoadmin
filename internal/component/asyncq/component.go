package asyncq

import (
	"context"
	"fmt"
	"time"

	"github.com/gotomicro/ego/core/elog"
	"github.com/hibiken/asynq"
)

const (
	// PackageName 包名
	PackageName = "component.asyncq"
)

// Component asyncq组件
type Component struct {
	name   string
	config *Config
	logger *elog.Component

	client *asynq.Client
	server *asynq.Server
	mux    *asynq.ServeMux
}

// newComponent 创建asyncq组件
func newComponent(name string, config *Config, logger *elog.Component) *Component {
	comp := &Component{
		name:   name,
		config: config,
		logger: logger,
		mux:    asynq.NewServeMux(),
	}

	// 初始化客户端
	if config.EnableClient {
		comp.client = asynq.NewClient(config.RedisConnOpt())
		comp.logger.Info("asyncq client initialized", elog.FieldAddr(config.RedisAddr))
	}

	// 初始化服务端
	if config.EnableServer {
		serverConfig := asynq.Config{
			Concurrency:    config.Concurrency,
			Queues:         config.Queues,
			RetryDelayFunc: comp.getRetryDelayFunc(),
			ErrorHandler:   asynq.ErrorHandlerFunc(comp.errorHandler),
		}
		comp.server = asynq.NewServer(config.RedisConnOpt(), serverConfig)
		comp.logger.Info("asyncq server initialized",
			elog.FieldAddr(config.RedisAddr),
			elog.Any("queues", config.Queues),
			elog.Int("concurrency", config.Concurrency))
	}

	return comp
}

// getRetryDelayFunc 获取重试延迟函数
func (c *Component) getRetryDelayFunc() asynq.RetryDelayFunc {
	switch c.config.RetryDelayFunc {
	case "exponential":
		return asynq.DefaultRetryDelayFunc
	case "linear":
		return func(n int, _ error, _ *asynq.Task) time.Duration {
			return time.Duration(n) * time.Second
		}
	case "fixed":
		return func(_ int, _ error, _ *asynq.Task) time.Duration {
			return 5 * time.Second
		}
	default:
		return asynq.DefaultRetryDelayFunc
	}
}

// errorHandler 错误处理器
func (c *Component) errorHandler(ctx context.Context, task *asynq.Task, err error) {
	c.logger.Error("task processing failed",
		elog.FieldErr(err),
		elog.String("task_type", task.Type()),
		elog.String("task_payload", string(task.Payload())))
}

// Enqueue 入队任务
func (c *Component) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("asyncq client not initialized")
	}
	start := time.Now()
	info, err := c.client.EnqueueContext(ctx, task, opts...)
	duration := time.Since(start)

	if c.config.EnableAccessLog {
		// info 可能为nil（若出错），谨慎访问
		taskID := ""
		queue := ""
		if info != nil {
			taskID = info.ID
			queue = info.Queue
		}
		c.logger.Info("enqueue task",
			elog.String("task_type", task.Type()),
			elog.String("task_id", taskID),
			elog.String("queue", queue),
			elog.Duration("duration", duration),
			elog.FieldErr(err))
	}
	if duration > c.config.SlowLogThreshold {
		c.logger.Warn("slow enqueue task",
			elog.String("task_type", task.Type()),
			elog.Duration("duration", duration))
	}
	return info, err
}

// EnqueueIn 延迟入队任务
func (c *Component) EnqueueIn(ctx context.Context, delay time.Duration, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("asyncq client not initialized")
	}
	start := time.Now()
	info, err := c.client.EnqueueContext(ctx, task, append(opts, asynq.ProcessIn(delay))...)
	duration := time.Since(start)

	if c.config.EnableAccessLog {
		taskID := ""
		queue := ""
		if info != nil {
			taskID = info.ID
			queue = info.Queue
		}
		c.logger.Info("enqueue delayed task",
			elog.String("task_type", task.Type()),
			elog.String("task_id", taskID),
			elog.String("queue", queue),
			elog.Duration("delay", delay),
			elog.Duration("duration", duration),
			elog.FieldErr(err))
	}
	if duration > c.config.SlowLogThreshold {
		c.logger.Warn("slow delayed enqueue task",
			elog.String("task_type", task.Type()),
			elog.Duration("delay", delay),
			elog.Duration("duration", duration))
	}
	return info, err
}

// EnqueueAt 定时入队任务
func (c *Component) EnqueueAt(ctx context.Context, processAt time.Time, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("asyncq client not initialized")
	}
	start := time.Now()
	info, err := c.client.EnqueueContext(ctx, task, append(opts, asynq.ProcessAt(processAt))...)
	duration := time.Since(start)

	if c.config.EnableAccessLog {
		taskID := ""
		queue := ""
		if info != nil {
			taskID = info.ID
			queue = info.Queue
		}
		c.logger.Info("enqueue scheduled task",
			elog.String("task_type", task.Type()),
			elog.String("task_id", taskID),
			elog.String("queue", queue),
			elog.Any("process_at", processAt),
			elog.Duration("duration", duration),
			elog.FieldErr(err))
	}
	if duration > c.config.SlowLogThreshold {
		c.logger.Warn("slow scheduled enqueue task",
			elog.String("task_type", task.Type()),
			elog.Any("process_at", processAt),
			elog.Duration("duration", duration))
	}
	return info, err
}

// RegisterHandler 注册任务处理器
func (c *Component) RegisterHandler(pattern string, handler asynq.Handler) {
	if c.mux == nil {
		c.logger.Error("asyncq server mux not initialized")
		return
	}
	wrapped := c.wrapHandler(pattern, handler)
	c.mux.Handle(pattern, wrapped)
	c.logger.Info("registered task handler", elog.String("pattern", pattern))
}

// RegisterHandlerFunc 注册任务处理函数
func (c *Component) RegisterHandlerFunc(pattern string, handler func(context.Context, *asynq.Task) error) {
	c.RegisterHandler(pattern, asynq.HandlerFunc(handler))
}

// wrapHandler 包装处理器用于日志与慢查询
func (c *Component) wrapHandler(pattern string, handler asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		start := time.Now()
		if c.config.EnableAccessLog {
			c.logger.Info("processing task",
				elog.String("pattern", pattern),
				elog.String("task_type", task.Type()),
				elog.String("task_payload", string(task.Payload())))
		}
		err := handler.ProcessTask(ctx, task)
		duration := time.Since(start)

		if c.config.EnableAccessLog {
			c.logger.Info("task processed",
				elog.String("pattern", pattern),
				elog.String("task_type", task.Type()),
				elog.Duration("duration", duration),
				elog.FieldErr(err))
		}
		if duration > c.config.SlowLogThreshold {
			c.logger.Warn("slow task processing",
				elog.String("pattern", pattern),
				elog.String("task_type", task.Type()),
				elog.Duration("duration", duration))
		}
		return err
	})
}

// Start 启动服务端
func (c *Component) Start() error {
	if c.server == nil {
		return fmt.Errorf("asyncq server not initialized")
	}
	c.logger.Info("starting asyncq server")
	return c.server.Start(c.mux)
}

// Stop 停止服务端（立即）
func (c *Component) Stop() error {
	if c.server == nil {
		return nil
	}
	c.logger.Info("stopping asyncq server")
	c.server.Stop()
	return nil
}

// Shutdown 优雅关闭服务端
func (c *Component) Shutdown() error {
	if c.server == nil {
		return nil
	}
	c.logger.Info("shutting down asyncq server")
	c.server.Shutdown()
	return nil
}

// Close 关闭组件
func (c *Component) Close() error {
	var err error
	// 关闭服务端
	if c.server != nil {
		c.server.Shutdown()
		c.logger.Info("asyncq server closed")
	}
	// 关闭客户端
	if c.client != nil {
		if closeErr := c.client.Close(); closeErr != nil {
			err = closeErr
			c.logger.Error("failed to close asyncq client", elog.FieldErr(closeErr))
		} else {
			c.logger.Info("asyncq client closed")
		}
	}
	return err
}

// Health 健康检查：若启用则尝试入队一个测试任务到 health 队列
func (c *Component) Health(ctx context.Context) error {
	if !c.config.EnableHealthCheck {
		return nil
	}
	if c.client != nil {
		testTask := asynq.NewTask("health_check", nil)
		if _, err := c.client.EnqueueContext(ctx, testTask, asynq.Queue("health")); err != nil {
			return fmt.Errorf("asyncq client health check failed: %v", err)
		}
	}
	return nil
}

// GetClient 获取客户端实例
func (c *Component) GetClient() *asynq.Client {
	return c.client
}

// GetServer 获取服务端实例
func (c *Component) GetServer() *asynq.Server {
	return c.server
}

// GetMux 获取路由器实例
func (c *Component) GetMux() *asynq.ServeMux {
	return c.mux
}
