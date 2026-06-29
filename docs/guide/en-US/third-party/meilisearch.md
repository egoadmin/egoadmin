# Search Service with MeiliSearch

EgoAdmin integrates MeiliSearch for full-text search capabilities, suitable for content retrieval, product search, and log queries.

## Overview

MeiliSearch is an open-source full-text search engine known for out-of-the-box functionality and low latency. EgoAdmin wraps it in `internal/component/meilisearch/` using an interface-provider pattern, injected into business services via Wire.

Core capabilities:

| Capability | Description |
|-----------|-------------|
| Index Management | Create, ensure, and query indexes |
| Document CRUD | Add, update, and delete documents in indexes |
| Full-Text Search | Fuzzy matching, prefix matching, filtering, sorting |
| Health Check | Optional health probes with slow-log warnings |
| Build-Time Index | Auto-ensure indexes exist on startup |

::: tip Why MeiliSearch
Compared to Elasticsearch, MeiliSearch is simpler to install and configure, offers lower search latency (typically < 50ms), and has built-in typo tolerance and facet search -- ideal for small-to-medium project search needs.
:::

## Core Usage

### Component Interface

The MeiliSearch component defines a clean interface for testability and substitution:

```go
// internal/component/meilisearch/interface.go

type Interface interface {
    // Health checks MeiliSearch health status
    Health(ctx context.Context) error

    // EnsureIndexes ensures indexes exist (skips if already present)
    EnsureIndexes(ctx context.Context, indexes []IndexConf) error

    // Client returns the underlying client (meilisearch.ServiceManager)
    Client() ms.ServiceManager

    // Close closes the client
    Close()
}
```

### Wire Injection

```go
// internal/component/meilisearch/provider.go

var ProviderSet = wire.NewSet(
    NewMeiliComponent,
)

// Loads from config key "client.meili" by default
func NewMeiliComponent() *Component {
    return Load("client.meili").Build()
}
```

Reference it in the service's Wire build:

```go
// internal/app/gateway/server/wire.go
func InitServer() *Server {
    wire.Build(
        meilisearch.ProviderSet,
        // ... other dependencies
    )
    return nil
}
```

### Index Management

Declare indexes in configuration; the component ensures they exist on startup:

```go
// Index configuration type
type IndexConf struct {
    Name       string // Index name
    PrimaryKey string // Primary key field
}
```

```toml
[client.meili]
indexes = [
    { name = "products", primaryKey = "id" },
    { name = "articles", primaryKey = "id" },
]
```

Manually ensure indexes:

```go
err := meiliComp.EnsureIndexes(ctx, []meilisearch.IndexConf{
    {Name: "products", PrimaryKey: "id"},
    {Name: "articles", PrimaryKey: "id"},
})
```

### Document Operations

Perform document CRUD via the underlying `ms.ServiceManager` client:

```go
client := meiliComp.Client()

// Add/update documents
task, err := client.Index("products").AddDocuments([]map[string]interface{}{
    {
        "id":    "1",
        "name":  "EgoAdmin T-Shirt",
        "price": 99.00,
        "tags":  []string{"clothing", "merchandise"},
    },
})

// Update a single document
task, err = client.Index("products").UpdateDocuments([]map[string]interface{}{
    {
        "id":    "1",
        "price": 79.00, // Discount
    },
})

// Delete a document
task, err = client.Index("products").DeleteDocument("1")

// Batch delete
task, err = client.Index("products").DeleteDocuments([]string{"1", "2", "3"})
```

### Search Queries

```go
client := meiliComp.Client()

// Basic full-text search
searchRes, err := client.Index("products").Search("t-shirt", &ms.SearchRequest{
    Limit: 20,
})

// With filtering and sorting
searchRes, err = client.Index("products").Search("t-shirt", &ms.SearchRequest{
    Filter: "price > 50 AND tags = clothing",
    Sort:   []string{"price:asc"},
    Limit:  20,
    Offset: 0,
})

// Specify return attributes
searchRes, err = client.Index("products").Search("t-shirt", &ms.SearchRequest{
    AttributesToRetrieve:  []string{"id", "name", "price"},
    AttributesToHighlight: []string{"name"},
    HighlightPreTag:       "<em>",
    HighlightPostTag:      "</em>",
})

// Iterate results
for _, hit := range searchRes.Hits {
    m := hit.(map[string]interface{})
    fmt.Printf("ID: %v, Name: %v\n", m["id"], m["name"])
}
```

### Field Weights and Search Features

```go
// Set searchable attributes with weights
task, err := client.Index("products").UpdateSearchableAttributes(&ms.SearchableAttributes{
    SearchableAttributes: []string{"name", "description", "tags"},
})

// Set filterable and sortable attributes
task, err = client.Index("products").UpdateFilterableAttributes(&ms.FilterableAttributes{
    FilterableAttributes: []string{"price", "tags", "category"},
})

task, err = client.Index("products").UpdateSortableAttributes(&ms.SortableAttributes{
    SortableAttributes: []string{"price", "createdAt"},
})

// Set synonyms
task, err = client.Index("products").UpdateSynonyms(&ms.Synonyms{
    Synonyms: map[string][]string{
        "tshirt":  {"t-shirt", "tee"},
        "laptop":  {"notebook", "computer"},
    },
})

// Set stop words
task, err = client.Index("products").UpdateStopWords(&ms.StopWords{
    StopWords: []string{"the", "a", "an", "of"},
})
```

## Configuration Examples

### Basic Configuration

```toml
[client.meili]
host = "http://127.0.0.1:7700"
apiKey = "egoadmin"
timeout = "5s"
enableHealth = true
ensureOnBuild = false
enableAccessLog = false
slowLog = "1s"

# Index declarations
[[client.meili.indexes]]
name = "products"
primaryKey = "id"

[[client.meili.indexes]]
name = "articles"
primaryKey = "id"
```

### Full Configuration

```toml
[client.meili]
host = "http://127.0.0.1:7700"
apiKey = "egoadmin-master-key"
timeout = "10s"
enableHealth = true
ensureOnBuild = true           # Auto-create indexes on startup
enableAccessLog = true         # Enable access logging
slowLog = "500ms"              # Slow query threshold
```

### Environment Variable Overrides

```bash
# MeiliSearch connection
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

## Real-World Examples

### Search Service Wrapper

Wrap search logic in the Application layer to avoid business code directly manipulating the MeiliSearch client:

```go
type ProductSearchService struct {
    meili meilisearch.Interface
}

func (s *ProductSearchService) Search(ctx context.Context, query string, page, size int) (*SearchResult, error) {
    client := s.meili.Client()

    res, err := client.Index("products").Search(query, &ms.SearchRequest{
        Limit:  int64(size),
        Offset: int64((page - 1) * size),
        AttributesToRetrieve:  []string{"id", "name", "price", "thumbnail"},
        AttributesToHighlight: []string{"name"},
    })
    if err != nil {
        return nil, fmt.Errorf("search products failed: %w", err)
    }

    return &SearchResult{
        Hits:      res.Hits,
        TotalHits: res.EstimatedTotalHits,
        QueryTime: res.ProcessingTimeMs,
    }, nil
}
```

### Incremental Sync

Event-driven index update pattern, processed asynchronously via AsyncQ:

```go
const TaskTypeProductIndexSync = "product:index_sync"

// Register task handler
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

// Enqueue sync task when a product changes
func (s *ProductService) OnProductUpdated(ctx context.Context, product *Product) error {
    payload, _ := json.Marshal(ProductSyncPayload{
        Action:    "upsert",
        Documents: []interface{}{product.ToSearchDoc()},
    })
    _, err := s.asyncq.Enqueue(ctx, asynq.NewTask(TaskTypeProductIndexSync, payload))
    return err
}
```

### Health Check Integration

```go
// Register with the health check framework
func (s *Server) initHealthChecks() {
    s.health.Register("meilisearch", func(ctx context.Context) error {
        return s.meili.Health(ctx)
    })
}
```

## How It Works

### Component Lifecycle

```text
Wire initialization
  |
  v
NewMeiliComponent()
  |-- Read config key "client.meili"
  |-- Create HTTP client (with timeout)
  |-- Initialize ms.New(host, apiKey)
  |-- If ensureOnBuild=true, auto-create indexes
  |
  v
Component ready
  |
  v
Business services use it via Wire injection
  |
  v
Close() shuts down the connection
```

### Search Request Flow

```text
Business code
  |
  v
meiliComp.Client().Index("xxx").Search(...)
  |
  v
MeiliSearch HTTP API (POST /indexes/{uid}/search)
  |
  v
MeiliSearch engine
  |-- Tokenization (multi-language support)
  |-- Typo tolerance (spelling correction)
  |-- Filtering + Sorting
  |-- Facet aggregation
  |
  v
Return results (with highlights, processing time, total hits)
```

### Slow Logging and Monitoring

The component has built-in slow-log detection. When an operation exceeds the `slowLog` threshold, a WARN-level log is emitted:

```text
WARN  slow ensure index   index=products  duration=1.5s
WARN  slow health check   duration=800ms
```

With `enableAccessLog = true`, detailed logs are recorded for every operation, useful for diagnosing performance issues.

## Common Issues

### Search Returns Empty Results

```text
hits: [], estimatedTotalHits: 0
```

Check these items:

1. Does the index exist? `curl http://127.0.0.1:7700/indexes`
2. Does the index have documents? `curl http://127.0.0.1:7700/indexes/products/documents`
3. Is the search term correct? (MeiliSearch is case-insensitive by default with typo tolerance)
4. Are filter conditions excluding results?

### MeiliSearch Connection Timeout

```text
context deadline exceeded
```

Solution:

```toml
# Increase timeout
[client.meili]
timeout = "30s"
```

Check that the MeiliSearch service is running and that the index is not undergoing a large document import (which consumes IO).

### Index Creation Failure

```text
meilisearch: bad request: index already exists
```

This is not an error. `EnsureIndexes` handles this case: it first queries whether the index exists and skips if so. If you see this log, it means `CreateIndex` was called directly instead of `EnsureIndexes`.

### Document Sync Delay

With async sync mode, search results may have brief delays. Solutions:

1. Sync critical operations (e.g., product listing) by calling the index API directly
2. Process non-critical operations (e.g., view count updates) asynchronously via AsyncQ
3. Use `WaitForTask` to wait for critical tasks to complete:

```go
task, err := client.Index("products").AddDocuments(docs)
// Wait for indexing to complete
_, err = client.WaitForTask(task.TaskUID, 5*time.Second)
```

### Out of Memory

MeiliSearch loads indexes into memory by default. For large data volumes:

```yaml
# docker-compose.yml environment
environment:
  MEILI_MAX_INDEXING_MEMORY: "2Gb"
```

## Reference Links

- [MeiliSearch Official Documentation](https://www.meilisearch.com/docs)
- [meilisearch-go SDK](https://github.com/meilisearch/meilisearch-go)
- [Search Parameter Reference](https://www.meilisearch.com/docs/reference/api/search)
- EgoAdmin source: `internal/component/meilisearch/`
