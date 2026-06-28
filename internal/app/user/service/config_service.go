package service

import (
	"context"
	"errors"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/coocood/freecache"
)

// ConfigService 系统设置服务
type ConfigService struct {
	Options
}

// NewConfigService 系统设置服务
func NewConfigService(options Options) *ConfigService {
	return &ConfigService{
		Options: options,
	}
}

// SetConfig 设置配置
func (s *ConfigService) SetConfig(ctx context.Context, key string, value string) (err error) {
	err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
		// 获取配置信息
		_, er := s.Config.Get(txCtx, key)
		// 判断是否找到配置信息
		hasFound := false
		if er == nil {
			hasFound = true
		}
		// 如果找不到配置信息，则将错误置为nil
		if xorm.IsErrRecordNotFound(er) {
			er = nil
		}
		// 如果出现其他错误，则直接返回
		if er != nil {
			return er
		}

		// 如果找到了配置信息，则更新配置信息
		if hasFound {
			return s.Config.Update(txCtx, key, value)
		}

		// 如果没有找到配置信息，则添加配置信息
		return s.Config.Add(txCtx, &store.ConfigModel{
			Ckey:  key,
			Value: value,
		})
	})
	if err != nil {
		return
	}
	err = s.ConfigCache.AddConfig(key, value)

	return
}

// GetConfig 获取配置
// GetConfig函数从缓存中获取配置信息，如果缓存中没有，则从数据库中获取并重新加载缓存
func (s *ConfigService) GetConfig(ctx context.Context, key string) (value string, err error) {
	// 从缓存中获取配置信息
	value, err = s.ConfigCache.GetConfig(key)
	// 判断是否成功获取到配置信息
	hasConfig := false
	if err == nil {
		hasConfig = true
	}
	// 如果缓存中没有该配置信息，则将err置为nil
	if errors.Is(err, freecache.ErrNotFound) {
		err = nil
	}
	// 如果获取配置信息出错，则直接返回
	if err != nil {
		return
	}

	// 如果缓存中有该配置信息，则直接返回
	if hasConfig {
		return
	}

	// 重新加载缓存
	// 从数据库中获取配置信息
	conf, err := s.Config.Get(ctx, key)
	// 如果该配置信息不存在，则将err置为nil，并将空字符串添加到缓存中
	if xorm.IsErrRecordNotFound(err) {
		// 空缓存
		err = s.ConfigCache.AddConfig(key, "")

		return
	}
	// 如果获取配置信息出错，则直接返回
	if err != nil {
		return
	}
	// 将获取到的配置信息添加到缓存中
	value = conf.Value
	err = s.ConfigCache.AddConfig(key, conf.Value)

	return
}
