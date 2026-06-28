package httpclient

import (
	"context"

	"github.com/google/wire"
	"github.com/gotomicro/ego/client/ehttp"
	"github.com/gotomicro/ego/core/elog"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(
	NewExample,
)

type Example struct {
	cc *ehttp.Component
}

type ExampleModel struct {
	gorm.Model
}

func NewExample() ExampleInterface {
	httpComp := ehttp.Load("client.http").Build()

	return &Example{
		cc: httpComp,
	}
}

func (s *Example) SayHello(ctx context.Context) (string, error) {
	res, err := s.cc.R().SetContext(ctx).Get("/")
	if err != nil {
		elog.Error("SayHello failed", elog.FieldErr(err))
		return "", err
	}

	return res.String(), nil
}
