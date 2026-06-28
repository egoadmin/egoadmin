package config

import "strings"

// Service 标识一个微服务，用于选择内置默认配置和服务专属解析。
type Service string

const (
	ServiceGateway Service = "gateway"
	ServiceUser    Service = "user"
	ServiceIDGen   Service = "idgen"
)

// normalizeService 归一化服务标识，空值或 gateway 归一为 ServiceGateway。
func normalizeService(service Service) Service {
	switch strings.TrimSpace(strings.ToLower(string(service))) {
	case "", "gateway":
		return ServiceGateway
	case "user":
		return ServiceUser
	case "idgen":
		return ServiceIDGen
	default:
		return service
	}
}
