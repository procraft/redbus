package main

import (
	"context"
	"log"

	"github.com/sergiusd/redbus/internal/app/config"
	"github.com/sergiusd/redbus/internal/pkg/app"
)

func main() {
	ctx := context.Background()
	conf := config.New()

	redbus, err := app.New(ctx, conf)
	if err != nil {
		log.Fatalf(err.Error())
	}

	if err := redbus.Run(); err != nil {
		log.Fatalf(err.Error())
	}
}
