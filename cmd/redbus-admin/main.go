package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/procraft/redbus/internal/config"
	"github.com/procraft/redbus/internal/pkg/adminapp"
	"github.com/procraft/redbus/internal/pkg/logger"
)

func main() {
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	// Its own app label: the admin runs as a separate deployment, and {app=~"redbus.*"} still
	// selects both processes.
	lokiConf, err := conf.Log.Loki.Logger("redbus-admin")
	if err != nil {
		log.Fatalln(err)
	}
	stopLoki := logger.StartLoki(lokiConf)
	fatal := func(err error) {
		logger.Error(logger.App, "%v", err)
		stopLogs(stopLoki)
		log.Fatalln(err)
	}

	app, err := adminapp.New(conf)
	if err != nil {
		fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fatal(err)
	}
	stopLogs(stopLoki)
}

// stopLogs pushes the queued log lines before the process exits.
func stopLogs(stop func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stop(ctx)
}
