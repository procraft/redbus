package main

import (
	"context"
	"errors"
	"github.com/prokraft/redbus/internal/config"
	"log"

	"github.com/prokraft/redbus/internal/pkg/app"
)

func main() {
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()
	redbus, err := app.New(ctx, conf)
	if err != nil {
		log.Fatalf(err.Error())
	}

	if err := redbus.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Fatalf(err.Error())
		}
	}
}
