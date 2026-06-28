package example

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	stdjetcache "github.com/mgtv-tech/jetcache-go"
)

// ExampleUsage 展示jetcache组件使用示例
func ExampleUsage() {
	// 1. 初始化配置
	ed := eredis.Load("redis.eredis").Build()
	// 2. 创建Redis客户端

	// 3. 加载jetcache组件
	cache := jetcache.Load("cache.jetcache").Build(jetcache.WithEredis(ed))

	// 4. 使用缓存示例
	ctx := context.Background()

	// 示例1: 设置缓存
	user := map[string]interface{}{
		"id":    1,
		"name":  "张三",
		"email": "zhangsan@example.com",
	}
	err := cache.Cache().Set(ctx, "user:1", stdjetcache.Value(user), stdjetcache.TTL(2*time.Hour))
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
	}

	// 示例2: 获取缓存
	var cachedUser map[string]interface{}
	err = cache.Cache().Get(ctx, "user:1", &cachedUser)
	if err != nil {
		log.Printf("获取缓存失败: %v", err)
	} else {
		fmt.Printf("获取到的用户: %+v\n", cachedUser)
	}

	// 示例3: 使用Once接口（防缓存击穿）
	productKey := "product:1001"
	var product map[string]interface{}
	err = cache.Cache().Once(ctx, productKey,
		stdjetcache.Value(&product),
		stdjetcache.Do(func(ctx context.Context) (interface{}, error) {
			// 模拟从数据库获取数据
			fmt.Println("缓存未命中，从数据库加载数据...")
			return map[string]interface{}{
				"id":    1001,
				"name":  "示例产品",
				"price": 99.99,
			}, nil
		}),
	)
	if err != nil {
		log.Printf("Once操作失败: %v", err)
	} else {
		fmt.Printf("获取到的产品: %+v\n", product)
	}

	// 示例4: 检查缓存是否存在
	exists := cache.Cache().Exists(ctx, "user:1")
	fmt.Printf("缓存user:1是否存在: %v\n", exists)

	// 示例5: 删除缓存
	err = cache.Cache().Delete(ctx, "user:1")
	if err != nil {
		log.Printf("删除缓存失败: %v", err)
	} else {
		fmt.Println("删除缓存成功")
	}

	// 示例6: 删除缓存
	err = cache.Cache().Delete(ctx, "user:1")
	if err != nil {
		log.Printf("删除缓存失败: %v", err)
	}

	// 示例6: 使用JSON序列化复杂对象
	type ComplexObject struct {
		ID      int                `json:"id"`
		Name    string             `json:"name"`
		Tags    []string           `json:"tags"`
		Metrics map[string]float64 `json:"metrics"`
	}

	complexObj := ComplexObject{
		ID:   1001,
		Name: "复杂对象",
		Tags: []string{"tag1", "tag2", "tag3"},
		Metrics: map[string]float64{
			"cpu":    80.5,
			"memory": 45.2,
		},
	}

	err = cache.Cache().Set(ctx, "complex:1001", stdjetcache.Value(complexObj), stdjetcache.TTL(30*time.Minute))
	if err != nil {
		log.Printf("设置复杂对象缓存失败: %v", err)
	}

	var retrievedObj ComplexObject
	err = cache.Cache().Get(ctx, "complex:1001", &retrievedObj)
	if err != nil {
		log.Printf("获取复杂对象缓存失败: %v", err)
	} else {
		fmt.Printf("获取到的复杂对象: %+v\n", retrievedObj)
	}

	// 示例7: 查看缓存类型和任务信息
	cacheType := cache.Cache().CacheType()
	taskSize := cache.Cache().TaskSize()
	fmt.Printf("缓存类型: %s, 刷新任务数量: %d\n", cacheType, taskSize)

	// 示例9: 清理所有缓存（谨慎使用）
	// err = cache.Flush(ctx)
	// if err != nil {
	//     log.Printf("清理缓存失败: %v", err)
	// }

	fmt.Println("jetcache示例执行完成")
}

// ExampleWithConfig 展示使用配置文件的方式
func ExampleWithConfig() {
	// 1. 从配置文件加载
	cacheComponent := jetcache.Load("cache.jetcache").Build()
	cache := cacheComponent.Cache()

	ctx := context.Background()

	// 2. 使用缓存
	data := map[string]string{
		"message": "Hello, JetCache!",
		"time":    time.Now().Format(time.RFC3339),
	}

	err := cache.Set(ctx, "greeting", stdjetcache.Value(data), stdjetcache.TTL(10*time.Minute))
	if err != nil {
		log.Printf("设置缓存失败: %v", err)
		return
	}

	var result map[string]string
	err = cache.Get(ctx, "greeting", &result)
	if err != nil {
		log.Printf("获取缓存失败: %v", err)
		return
	}

	fmt.Printf("获取到的数据: %+v\n", result)
}

// ExampleAdvancedUsage 高级用法示例
func ExampleAdvancedUsage() {
	// 高级配置
	cacheComponent := jetcache.Load("cache.jetcache").Build()
	cache := cacheComponent.Cache()

	ctx := context.Background()

	// 使用带自动刷新的缓存
	configKey := "app:config"
	var appConfig map[string]interface{}

	err := cache.Once(ctx, configKey,
		stdjetcache.Value(&appConfig),
		stdjetcache.Refresh(true), // 启用自动刷新
		stdjetcache.Do(func(ctx context.Context) (interface{}, error) {
			// 从配置中心获取最新配置
			return map[string]interface{}{
				"version":  "1.0.0",
				"features": []string{"auth", "cache", "logging"},
				"settings": map[string]interface{}{
					"timeout": 30,
					"retries": 3,
				},
			}, nil
		}),
	)
	if err != nil {
		log.Printf("获取配置失败: %v", err)
		return
	}

	fmt.Printf("应用配置: %+v\n", appConfig)

	// 查看缓存类型和任务信息
	cacheType := cache.CacheType()
	taskSize := cache.TaskSize()
	fmt.Printf("缓存类型: %s, 刷新任务数量: %d\n", cacheType, taskSize)

	fmt.Println("高级示例执行完成")
}
