//go:build ignore

// 该文件仅作为 asyncq 组件的使用示例，默认不参与构建。
// 如需运行示例，请移除第一行的构建标签或使用自定义 build tag。

package main

import (
	"context"
	"log"
	"time"

	"github.com/hibiken/asynq"

	"github.com/egoadmin/egoadmin/internal/component/asyncq"
)

// 示例：加载 asyncq 组件，入队任务，并注册处理器启动服务端。
// 注意：示例仅演示用法，实际项目中请在 app 初始化阶段统一管理组件生命周期。
func main() {
	// 1. 从配置加载组件（默认键 client.asyncq，见 provider.go）
	//    也可使用 asyncq.Load("your.key").Build(asyncq.WithRedisAddr("127.0.0.1:6379")) 自定义。
	comp := asyncq.Load("client.asyncq").Build(
	// 示例：覆盖部分配置
	// asyncq.WithEnableClient(true),
	// asyncq.WithEnableServer(true),
	)
	defer func() { _ = comp.Close() }()

	ctx := context.Background()

	// 2. 客户端：构造任务并入队
	if cl := comp.GetClient(); cl != nil {
		task := asynq.NewTask("demo:print", []byte(`{"msg":"hello asyncq"}`))
		info, err := comp.Enqueue(ctx, task, asynq.Queue("default"))
		if err != nil {
			log.Printf("enqueue error: %v\n", err)
		} else {
			log.Printf("enqueue ok: id=%s queue=%s type=%s\n", info.ID, info.Queue, info.Type)
		}
	}

	// 3. 服务端：注册处理器并启动（需启用 EnableServer）
	if srv := comp.GetServer(); srv != nil {
		comp.RegisterHandlerFunc("demo:print", func(ctx context.Context, t *asynq.Task) error {
			log.Printf("processing task type=%s payload=%s\n", t.Type(), string(t.Payload()))
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		// 启动服务（阻塞运行）；示例中运行 2 秒后停止
		go func() {
			if err := comp.Start(); err != nil {
				log.Printf("server start error: %v\n", err)
			}
		}()

		time.Sleep(2 * time.Second)
		_ = comp.Shutdown()
	}
}
