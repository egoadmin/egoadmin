package etusupload

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/gin-gonic/gin"
)

// ExampleBasicUsage 基础使用示例
func ExampleBasicUsage() {
	// 1. 加载并构建组件
	comp := Load("component.etusupload").Build()

	// 2. 创建Gin引擎
	engine := gin.Default()

	// 3. 添加CORS中间件
	engine.Use(NewCorsMiddleware(comp.GetConfig()))

	// 4. 注册TUS路由
	if err := comp.RegisterRoutes(engine); err != nil {
		log.Printf("register routes failed: %v", err)
		return
	}

	// 5. 启动服务
	if err := engine.Run(":8080"); err != nil {
		log.Printf("run server failed: %v", err)
	}
}

// ExampleWithValidator 自定义验证器示例
func ExampleWithValidator() {
	comp := Load("component.etusupload").Build()

	// 创建自定义验证器
	validator := NewDefaultValidator().
		WithAllowedExtensions([]string{".jpg", ".png", ".pdf"}).
		WithRejectedExtensions([]string{".exe", ".zip"})

	comp.RegisterValidator(validator)

	engine := gin.Default()
	engine.Use(NewCorsMiddleware(comp.GetConfig()))
	if err := comp.RegisterRoutes(engine); err != nil {
		log.Printf("register routes failed: %v", err)
		return
	}
	if err := engine.Run(":8080"); err != nil {
		log.Printf("run server failed: %v", err)
	}
}

// ExampleWithHook 使用钩子的示例
func ExampleWithHook() {
	comp := Load("component.etusupload").Build()

	// 注册上传钩子
	comp.RegisterHook(&MyUploadHook{})

	engine := gin.Default()
	engine.Use(NewCorsMiddleware(comp.GetConfig()))
	if err := comp.RegisterRoutes(engine); err != nil {
		log.Printf("register routes failed: %v", err)
		return
	}
	if err := engine.Run(":8080"); err != nil {
		log.Printf("run server failed: %v", err)
	}
}

// MyUploadHook 自定义上传钩子
type MyUploadHook struct{}

func (h *MyUploadHook) OnBeforeUpload(ctx context.Context, filename string) error {
	log.Printf("Before upload: %s", filename)
	return nil
}

func (h *MyUploadHook) OnAfterUpload(ctx context.Context, info *UploadInfo) error {
	log.Printf("After upload: %s -> %s", info.FileName, info.FilePath)
	// 可以在这里保存到数据库、触发处理流程等
	return nil
}

func (h *MyUploadHook) OnValidationFailed(ctx context.Context, filename string, err error) error {
	log.Printf("Validation failed for %s: %v", filename, err)
	return nil
}

// ExampleWithRouterGroup 使用路由组的示例
func ExampleWithRouterGroup() {
	comp := Load("component.etusupload").Build()

	engine := gin.Default()
	engine.Use(NewCorsMiddleware(comp.GetConfig()))

	// 注册到路由组
	api := engine.Group("/api")
	if err := comp.RegisterRoutesWithGroup(api, "/upload"); err != nil {
		log.Printf("register routes failed: %v", err)
		return
	}

	if err := engine.Run(":8080"); err != nil {
		log.Printf("run server failed: %v", err)
	}
}

// CustomValidator 完整的自定义验证器示例
type CustomValidator struct{}

func (v *CustomValidator) ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error {
	// 检查用户ID
	if _, ok := metadata["user_id"]; !ok {
		return fmt.Errorf("user_id is required")
	}
	return nil
}

func (v *CustomValidator) ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error {
	// 可以检查文件内容、病毒扫描等
	log.Printf("Validating file: %s", filename)
	return nil
}

// ExampleWithCustomValidator 完整的自定义验证器示例
func ExampleWithCustomValidator() {
	comp := Load("component.etusupload").Build()

	// 使用自定义验证器
	comp.RegisterValidator(&CustomValidator{})
	comp.RegisterHook(&MyUploadHook{})

	engine := gin.Default()
	engine.Use(NewCorsMiddleware(comp.GetConfig()))
	if err := comp.RegisterRoutes(engine); err != nil {
		log.Printf("register routes failed: %v", err)
		return
	}
	if err := engine.Run(":8080"); err != nil {
		log.Printf("run server failed: %v", err)
	}
}
