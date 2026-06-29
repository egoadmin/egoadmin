# 搜索服务 MeiliSearch

EgoAdmin 通过 MeiliSearch 组件提供全文搜索能力，适用于内容检索、商品搜索和日志查询等场景。

## 概述

MeiliSearch 是一个开源的全文搜索引擎，以开箱即用和低延迟著称。EgoAdmin 将其封装在 `internal/component/meilisearch/` 中，采用接口-Provider 模式，通过 Wire 依赖注入到业务服务。

核心能力：

| 能力 | 说明 |
|------|------|
| 索引管理 | 创建、确保、查询索引 |
| 文档 CRUD | 添加、更新、删除索引中的文档 |
| 全文搜索 | 模糊匹配、前缀匹配、过滤、排序 |
| 健康检查 | 可选的健康探活，带慢日志告警 |
| 构建时索引 | 启动时自动确保索引存在 |

::: tip 为什么用 MeiliSearch
相比 Elasticsearch，MeiliSearch 安装和配置更简单，搜索延迟更低（通常 < 50ms），内置 typo tolerance 和 facet search，适合中小型项目的搜索需求。
:::

## 核心用法

### 组件接口

MeiliSearch 组件定义了清晰的接口，方便测试和替换：

```go
// internal/component/meilisearch/interface.go

type Interface interface {
    // Health 检查 MeiliSearch 健康状态
    Health(ctx context.Context) error

    // EnsureIndexes 确保索引存在（存在则跳过）
    EnsureIndexes(ctx context.Context, indexes []IndexConf) error

    // Client 返回底层客户端（meilisearch.ServiceManager）
    Client() ms.ServiceManager

    // Close 关闭客户端
    Close()
}
```

### Wire 注入

```go
// internal/component/meilisearch/provider.go

var ProviderSet = wire.NewSet(
    NewMeiliComponent,
)

// 默认从配置键 client.meili 加载
func NewMeiliComponent() *Component {
    return Load("client.meili").Build()
}
```

在服务的 Wire 构建中引用：

```go
// internal/app/gateway/server/wire.go
func InitServer() *Server {
    wire.Build(
        meilisearch.ProviderSet,
        // ... 其他依赖
    )
    return nil
}
```

### 索引管理

通过配置声明索引，组件启动时自动确保索引存在：

```go
// 索引配置类型
type IndexConf struct {
    Name       string // 索引名称
    PrimaryKey string // 主键字段
}
```

```toml
[client.meili]
indexes = [
    { name = "products", primaryKey = "id" },
    { name = "articles", primaryKey = "id" },
]
```

手动确保索引：

```go
err := meiliComp.EnsureIndexes(ctx, []meilisearch.IndexConf{
    {Name: "products", PrimaryKey: "id"},
    {Name: "articles", PrimaryKey: "id"},
})
```

### 文档操作

通过底层 `ms.ServiceManager` 客户端执行文档 CRUD：

```go
client := meiliComp.Client()

// 添加/更新文档
task, err := client.Index("products").AddDocuments([]map[string]interface{}{
    {
        "id":    "1",
        "name":  "EgoAdmin T-Shirt",
        "price": 99.00,
        "tags":  []string{"clothing", "merchandise"},
    },
})

// 更新单个文档
task, err = client.Index("products").UpdateDocuments([]map[string]interface{}{
    {
        "id":    "1",
        "price": 79.00, // 折扣
    },
})

// 删除文档
task, err = client.Index("products").DeleteDocument("1")

// 批量删除
task, err = client.Index("products").DeleteDocuments([]string{"1", "2", "3"})
```

### 搜索查询

```go
client := meiliComp.Client()

// 基础全文搜索
searchRes, err := client.Index("products").Search("t-shirt", &ms.SearchRequest{
    Limit: 20,
})

// 带过滤和排序
searchRes, err = client.Index("products").Search("t-shirt", &ms.SearchRequest{
    Filter: "price > 50 AND tags = clothing",
    Sort:   []string{"price:asc"},
    Limit:  20,
    Offset: 0,
})

// 指定返回属性
searchRes, err = client.Index("products").Search("t-shirt", &ms.SearchRequest{
    AttributesToRetrieve: []string{"id", "name", "price"},
    AttributesToHighlight: []string{"name"},
    HighlightPreTag:  "<em>",
    HighlightPostTag: "</em>",
})

// 遍历结果
for _, hit := range searchRes.Hits {
    m := hit.(map[string]interface{})
    fmt.Printf("ID: %v, Name: %v\n", m["id"], m["name"])
}
```

### 字段权重与搜索特性

```go
// 设置可搜索属性的权重
task, err := client.Index("products").UpdateSearchableAttributes(&ms.SearchableAttributes{
    SearchableAttributes: []string{"name", "description", "tags"},
})

// 设置过滤和排序属性
task, err = client.Index("products").UpdateFilterableAttributes(&ms.FilterableAttributes{
    FilterableAttributes: []string{"price", "tags", "category"},
})

task, err = client.Index("products").UpdateSortableAttributes(&ms.SortableAttributes{
    SortableAttributes: []string{"price", "createdAt"},
})

// 设置同义词
task, err = client.Index("products").UpdateSynonyms(&ms.Synonyms{
    Synonyms: map[string][]string{
        "tshirt":  {"t-shirt", "tee"},
        "laptop":  {"notebook", "computer"},
    },
})

// 设置停用词
task, err = client.Index("products").UpdateStopWords(&ms.StopWords{
    StopWords: []string{"the", "a", "an", "of"},
})
```

## 配置示例

### 基础配置

```toml
[client.meili]
host = "http://127.0.0.1:7700"
apiKey = "egoadmin"
timeout = "5s"
enableHealth = true
ensureOnBuild = false
enableAccessLog = false
slowLog = "1s"

# 索引声明
[[client.meili.indexes]]
name = "products"
primaryKey = "id"

[[client.meili.indexes]]
name = "articles"
primaryKey = "id"
```

### 完整配置

```toml
[client.meili]
host = "http://127.0.0.1:7700"
apiKey = "egoadmin-master-key"
timeout = "10s"
enableHealth = true
ensureOnBuild = true           # 启动时自动创建索引
enableAccessLog = true         # 开启访问日志
slowLog = "500ms"              # 慢查询阈值
```

### 环境变量覆盖

```bash
# MeiliSearch 连接
EGOADMIN_CLIENT_MEILI_HOST=http://meili.prod.example.com:7700
EGOADMIN_CLIENT_MEILI_APIKEY=prod-master-key
EGOADMIN_CLIENT_MEILI_TIMEOUT=10s
```

### Docker Compose

```yaml
# test/compose/docker-compose.yml
services:
  meilisearch:
    image: getmeili/meilisearch:v1.7
    ports:
      - "7700:7700"
    environment:
      MEILI_MASTER_KEY: "egoadmin"
      MEILI_ENV: "development"
    volumes:
      - meili_data:/meili_data

volumes:
  meili_data:
```

## 实战示例

### 搜索服务封装

在 Application 层封装搜索逻辑，避免业务代码直接操作 MeiliSearch 客户端：

```go
type ProductSearchService struct {
    meili meilisearch.Interface
}

func (s *ProductSearchService) Search(ctx context.Context, query string, page, size int) (*SearchResult, error) {
    client := s.meili.Client()

    res, err := client.Index("products").Search(query, &ms.SearchRequest{
        Limit:  int64(size),
        Offset: int64((page - 1) * size),
        AttributesToRetrieve: []string{"id", "name", "price", "thumbnail"},
        AttributesToHighlight: []string{"name"},
    })
    if err != nil {
        return nil, fmt.Errorf("search products failed: %w", err)
    }

    return &SearchResult{
        Hits:       res.Hits,
        TotalHits:  res.EstimatedTotalHits,
        QueryTime:  res.ProcessingTimeMs,
    }, nil
}
```

### 增量同步

事件驱动的索引更新模式，通过 AsyncQ 异步处理：

```go
const TaskTypeProductIndexSync = "product:index_sync"

// 注册任务处理器
asyncq.RegisterHandlerFunc(TaskTypeProductIndexSync, func(ctx context.Context, task *asynq.Task) error {
    var payload ProductSyncPayload
    if err := json.Unmarshal(task.Payload(), &payload); err != nil {
        return err
    }

    client := meili.Client()
    switch payload.Action {
    case "upsert":
        _, err := client.Index("products").AddDocuments(payload.Documents)
        return err
    case "delete":
        _, err := client.Index("products").DeleteDocuments(payload.IDs)
        return err
    }
    return nil
})

// 商品变更时入队同步任务
func (s *ProductService) OnProductUpdated(ctx context.Context, product *Product) error {
    payload, _ := json.Marshal(ProductSyncPayload{
        Action:    "upsert",
        Documents: []interface{}{product.ToSearchDoc()},
    })
    _, err := s.asyncq.Enqueue(ctx, asynq.NewTask(TaskTypeProductIndexSync, payload))
    return err
}
```

### 健康检查集成

```go
// 注册到健康检查框架
func (s *Server) initHealthChecks() {
    s.health.Register("meilisearch", func(ctx context.Context) error {
        return s.meili.Health(ctx)
    })
}
```

## 工作原理

### 组件生命周期

```text
Wire 初始化
  |
  v
NewMeiliComponent()
  |-- 读取配置键 "client.meili"
  |-- 创建 HTTP 客户端（带超时）
  |-- 初始化 ms.New(host, apiKey)
  |-- 如果 ensureOnBuild=true，自动创建索引
  |
  v
组件就绪
  |
  v
业务服务通过 Wire 注入使用
  |
  v
Close() 关闭连接
```

### 搜索请求流

```text
业务代码
  |
  v
meiliComp.Client().Index("xxx").Search(...)
  |
  v
MeiliSearch HTTP API (POST /indexes/{uid}/search)
  |
  v
MeiliSearch 引擎
  |-- 分词（多语言支持）
  |-- typo tolerance（拼写纠错）
  |-- 过滤 + 排序
  |-- facet 聚合
  |
  v
返回结果（含高亮、处理时间、总命中数）
```

### 慢日志与监控

组件内置了慢日志检测。当操作耗时超过 `slowLog` 阈值时，输出 WARN 级别日志：

```text
WARN  slow ensure index   index=products  duration=1.5s
WARN  slow health check   duration=800ms
```

配合 `enableAccessLog = true` 可以记录每次操作的详细日志，用于排查性能问题。

## 常见问题

### 搜索返回空结果

```text
hits: [], estimatedTotalHits: 0
```

检查项：

1. 索引是否存在：`curl http://127.0.0.1:7700/indexes`
2. 索引中是否有文档：`curl http://127.0.0.1:7700/indexes/products/documents`
3. 搜索词是否正确（MeiliSearch 默认区分大小写但支持 typo tolerance）
4. 是否设置了过滤条件导致结果被排除

### MeiliSearch 连接超时

```text
context deadline exceeded
```

解决：

```toml
# 增加超时时间
[client.meili]
timeout = "30s"
```

检查 MeiliSearch 服务是否正常运行，索引是否在做大量文档导入（会占用 IO）。

### 索引创建失败

```text
meilisearch: bad request: index already exists
```

这不是错误。`EnsureIndexes` 已处理这种情况：先查询索引是否存在，存在则跳过。如果看到此日志，说明直接调用了 `CreateIndex` 而非 `EnsureIndexes`。

### 文档同步延迟

异步同步模式下，搜索结果可能有短暂延迟。解决方案：

1. 同步关键操作（如商品上架）直接调用索引 API
2. 非关键操作（如浏览量更新）通过 AsyncQ 异步处理
3. 使用 `WaitForTask` 等待关键任务完成：

```go
task, err := client.Index("products").AddDocuments(docs)
// 等待索引完成
_, err = client.WaitForTask(task.TaskUID, 5*time.Second)
```

### 内存不足

MeiliSearch 默认将索引加载到内存。如果数据量大：

```yaml
# docker-compose.yml 环境变量
environment:
  MEILI_MAX_INDEXING_MEMORY: "2Gb"
```

## 参考链接

- [MeiliSearch 官方文档](https://www.meilisearch.com/docs)
- [meilisearch-go SDK](https://github.com/meilisearch/meilisearch-go)
- [搜索参数详解](https://www.meilisearch.com/docs/reference/api/search)
- EgoAdmin 源码：`internal/component/meilisearch/`
