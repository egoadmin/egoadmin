package meilisearch

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gotomicro/ego/core/elog"
	ms "github.com/meilisearch/meilisearch-go"
)

const (
	// PackageName 包名
	PackageName = "component.meilisearch"
)

// Component meilisearch组件
type Component struct {
	name   string
	config *Config
	logger *elog.Component

	client ms.ServiceManager
}

// newComponent 创建组件
func newComponent(name string, config *Config, logger *elog.Component) *Component {
	httpClient := &http.Client{Timeout: config.Timeout}
	client := ms.New(config.Host,
		ms.WithAPIKey(config.APIKey),
		ms.WithCustomClient(httpClient),
	)

	c := &Component{
		name:   name,
		config: config,
		logger: logger,
		client: client,
	}

	logger.Info("meilisearch client initialized", elog.FieldAddr(config.Host))

	// 可选：构建时确保索引
	if config.EnsureOnBuild && len(config.Indexes) > 0 {
		_ = c.EnsureIndexes(context.Background(), config.Indexes)
	}

	return c
}

// Client 返回底层客户端
func (c *Component) Client() ms.ServiceManager {
	return c.client
}

// Health 健康检查
func (c *Component) Health(ctx context.Context) error {
	if !c.config.EnableHealth {
		return nil
	}
	start := time.Now()
	_, err := c.client.Health()
	duration := time.Since(start)

	if c.config.EnableAccessLog {
		c.logger.Info("meilisearch health check", elog.Duration("duration", duration), elog.FieldErr(err))
	}
	if duration > c.config.SlowLog {
		c.logger.Warn("meilisearch slow health check", elog.Duration("duration", duration))
	}
	return err
}

// EnsureIndexes 确保索引存在（存在则跳过，不存在则创建）
func (c *Component) EnsureIndexes(ctx context.Context, indexes []IndexConf) error {
	for _, idx := range indexes {
		if idx.Name == "" {
			continue
		}
		start := time.Now()

		_, err := c.client.GetIndex(idx.Name)
		if err == nil {
			if c.config.EnableAccessLog {
				c.logger.Info("index exists", elog.String("index", idx.Name))
			}
			continue
		}
		var meiliErr *ms.Error
		if !errors.As(err, &meiliErr) || meiliErr.MeilisearchApiError.Code != "index_not_found" {
			return err
		}

		task, err := c.client.CreateIndex(&ms.IndexConfig{
			Uid:        idx.Name,
			PrimaryKey: idx.PrimaryKey,
		})
		if err != nil {
			return err
		}

		if c.config.EnableAccessLog {
			c.logger.Info("ensure index", elog.String("index", idx.Name), elog.String("primaryKey", idx.PrimaryKey), elog.FieldErr(err))
		}

		// 这里只创建任务，不强制等待完成；如需等待可使用 c.client.WaitForTask/WithContext
		if err == nil && task != nil {
			c.logger.Info("ensure index task created", elog.String("index", idx.Name))
		}

		duration := time.Since(start)
		if duration > c.config.SlowLog {
			c.logger.Warn("slow ensure index", elog.String("index", idx.Name), elog.Duration("duration", duration))
		}
	}
	return nil
}

// Close 关闭组件
func (c *Component) Close() {
	if c.client == nil {
		return
	}

	c.client.Close()
}
