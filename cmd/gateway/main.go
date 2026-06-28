package main

import (
	"fmt"
	"os"

	"github.com/egoadmin/egoadmin/internal/app/gateway/server"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gotomicro/ego/core/elog"
)

func main() {
	if handleOfflineCommand(os.Args[1:]) {
		return
	}

	app, err := server.NewApp()
	if err != nil {
		panic(err)
	}

	if err = app.Run(); err != nil {
		elog.Panic("startup", elog.Any("err", err))
	}
}

func handleOfflineCommand(args []string) bool {
	if len(args) == 2 && args[0] == "config" && args[1] == "print-default" {
		fmt.Print(config.DefaultConfigDocument(config.DefaultEnvPrefix, config.ServiceGateway))
		return true
	}
	return false
}
