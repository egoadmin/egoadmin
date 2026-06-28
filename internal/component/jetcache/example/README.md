# JetCache 示例

本目录包含 JetCache 组件的使用示例。

## 示例列表

### 1. 基础用法 (example.go)

展示 JetCache 的基本使用方法：

- 缓存设置和获取
- 使用 Once 接口防止缓存击穿
- 批量操作（MSet/MGet）
- 复杂对象的序列化
- 缓存统计信息

### 2. 配置文件方式 (ExampleWithConfig)

展示如何通过配置文件使用 JetCache：

```go
cache := jetcache.Load("cache.jetcache").Build(jetcache.WithEredis(ed))
```

### 3. 高级用法 (ExampleAdvancedUsage)

展示 JetCache 的高级特性：

- 自动刷新缓存
- 统计功能启用
- 自定义本地缓存大小
- 命中率统计

## 运行示例

### 前提条件

1. 确保 Redis 服务运行在 localhost:6379
2. 配置文件中包含 jetcache 相关配置

### 运行方式

```bash
# 进入示例目录
cd internal/component/jetcache/example

# 运行示例
go run example.go
```

## 特性说明

### 1. 二级缓存
- 本地内存缓存（FreeCache）
- 分布式 Redis 缓存
- 自动缓存穿透保护

### 2. 智能缓存
- 自动刷新过期缓存
- 批量操作优化
- 统计和监控

### 3. 易用性
- 简单的 API 设计
- 支持复杂对象序列化
- 完整的错误处理

## 最佳实践

1. **缓存键设计**：使用有意义的键名，如 `user:{id}`、`product:{category}:{id}`
2. **缓存时间**：根据数据更新频率设置合适的 TTL
3. **批量操作**：对相关数据使用 MSet/MGet 提高性能
4. **监控统计**：定期检查缓存命中率和性能指标
5. **错误处理**：妥善处理缓存操作失败的情况

## 注意事项

1. 确保 Redis 服务可用性
2. 监控本地缓存内存使用情况
3. 定期检查缓存统计，优化缓存策略
4. 对于重要数据，考虑缓存穿透保护机制
