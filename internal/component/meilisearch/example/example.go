//go:build ignore

// 该文件为 meilisearch 组件使用示例，默认不参与构建。
// 如需运行示例，请移除第一行的构建标签。

package main

import (
	"context"
	"log"
	"time"

	ms "github.com/meilisearch/meilisearch-go"

	"github.com/egoadmin/egoadmin/internal/component/meilisearch"
)

func main() {
	// 从配置加载（默认键 client.meili），也可用可选项覆盖
	comp := meilisearch.Load("client.meili").Build(
	// meilisearch.WithHost("http://127.0.0.1:7700"),
	// meilisearch.WithAPIKey("masterKey"),
	// meilisearch.WithEnsureOnBuild(true),
	// meilisearch.WithIndexes([]meilisearch.IndexConf{{Name: "books", PrimaryKey: "id"}}),
	)
	defer func() { comp.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 健康检查
	if err := comp.Health(ctx); err != nil {
		log.Printf("health error: %v\n", err)
	}

	// 确保索引
	if err := comp.EnsureIndexes(ctx, []meilisearch.IndexConf{
		{Name: "books", PrimaryKey: "id"},
	}); err != nil {
		log.Printf("ensure indexes error: %v\n", err)
	}

	// 使用原生 client（需要安装依赖：go get github.com/meilisearch/meilisearch-go）
	// comp.Client() 返回 any，这里演示如何进行类型断言后再调用。
	if cli := comp.Client(); cli != nil {
		_, _ = cli.Index("books").AddDocuments([]map[string]any{
			{"id": 1, "title": "The Hobbit"},
		}, ms.StringPtr("id"))
	}
}
