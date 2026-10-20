package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/procraft/redbus/internal/config"

	"github.com/procraft/redbus/internal/pkg/app"
	"github.com/procraft/redbus/internal/pkg/logger"
)

func main() {
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	logger.JsonLog = conf.Log.Json
	logger.Verbose = conf.Log.Verbose

	lokiConf, err := conf.Log.Loki.Logger("redbus")
	if err != nil {
		log.Fatalln(err)
	}
	stopLoki := logger.StartLoki(lokiConf)
	fatal := func(err error) {
		logger.Error(logger.App, "%v", err)
		stopLogs(stopLoki)
		log.Fatalln(err.Error())
	}

	ctx := context.Background()
	redbus, err := app.New(ctx, conf)
	if err != nil {
		fatal(err)
	}

	if err := redbus.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			fatal(err)
		}
	}
	stopLogs(stopLoki)
}

// stopLogs pushes the queued log lines before the process exits.
func stopLogs(stop func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stop(ctx)
}
