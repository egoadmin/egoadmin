package health

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/server/egin"
)

// CheckFn 服务健康检查方法
type CheckFn func() bool

// Option is health option.
type Option func(*Options)

// Options is a health config
type Options struct {
	cf    CheckFn
	mu    sync.RWMutex
	ready bool // 是否准备就绪
}

// Ready 设置服务准备就绪状态
func (h *Options) Ready() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = true
}

// NotReady 设置服务不可接收新流量状态，通常用于停机 drain 阶段。
func (h *Options) NotReady() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ready = false
}

// IsReady 返回当前 readiness 状态。
func (h *Options) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.ready
}

// healthz 服务健康检查.
func (h *Options) healthz(c *gin.Context) {
	if !h.cf() {
		c.JSON(http.StatusBadGateway, gin.H{})

		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

// readyz 服务就绪检查.
func (h *Options) readyz(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.ready {
		c.JSON(http.StatusOK, gin.H{})

		return
	}
	c.JSON(http.StatusBadGateway, gin.H{})
}

// Start 注册服务健康检查
func Start(fn CheckFn, c *egin.Component, opts ...Option) *Options {
	o := &Options{
		cf:    fn,
		ready: false,
	}
	for _, opt := range opts {
		opt(o)
	}

	// 服务就绪检查
	c.GET("/readyz", o.readyz)

	// 服务健康检查
	c.GET("/healthz", o.healthz)

	return o
}
