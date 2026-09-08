package main

import (
	"context"
	"errors"
	"log"

	"github.com/prokraft/redbus/internal/config"

	"github.com/prokraft/redbus/internal/pkg/app"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

func main() {
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	logger.JsonLog = conf.Log.Json
	logger.Verbose = conf.Log.Verbose

	ctx := context.Background()
	redbus, err := app.New(ctx, conf)
	if err != nil {
		log.Fatalln(err.Error())
	}

	if err := redbus.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Fatalln(err.Error())
		}
	}
}
