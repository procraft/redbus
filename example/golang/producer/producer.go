package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/prokraft/redbus/api/golang/producer"
)

const dataBusServerPort = 50005

func main() {
	var topic string
	var key string
	var message string

	flag.StringVar(&topic, "t", "", "Topic name")
	flag.StringVar(&key, "k", "", "Message key")
	flag.StringVar(&message, "m", "", "Message content")
	flag.Parse()

	p, err := producer.New("", dataBusServerPort)
	if err != nil {
		panic(err.Error())
	}
	if err := p.Produce(context.Background(), topic, []byte(message), producer.WithKeyUUIDv4()); err != nil {
		panic(err.Error())
	}

	fmt.Println("Sent")
}
