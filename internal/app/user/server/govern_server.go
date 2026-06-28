package server

import "github.com/gotomicro/ego/server/egovernor"

func NewGovernServer() *egovernor.Component {
	return egovernor.Load("server.governor").Build()
}
