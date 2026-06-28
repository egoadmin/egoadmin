package server

import (
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gotomicro/ego/server/egovernor"
)

func NewGovernServer(_ config.EgoReady) *egovernor.Component {
	return egovernor.Load("server.governor").Build()
}
