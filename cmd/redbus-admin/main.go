package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/adminapp"
)

func main() {
	conf, err := config.FromFileAndEnv("./config.json", "./config.local.json")
	if err != nil {
		log.Fatalln(err)
	}

	app, err := adminapp.New(conf)
	if err != nil {
		log.Fatalln(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalln(err)
	}
}
