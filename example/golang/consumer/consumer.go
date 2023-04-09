package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/sergiusd/redbus/api/golang/consumer"
)

const dataBusServerPort = 50005
const dataBusUnavailableTimeout = time.Second * 5

func main() {
	var topic string
	var group string

	flag.StringVar(&topic, "t", "", "Topic name")
	flag.StringVar(&group, "g", "", "Group name")
	flag.Parse()

	if topic == "" || group == "" {
		log.Fatalln("Usage: example -t TOPIC -g GROUP")
	}

	c := consumer.New("localhost", dataBusServerPort,
		consumer.WithUnavailableTimeout(dataBusUnavailableTimeout),
		consumer.WithRepeatStrategyEven(10, 1),
	)
	if err := c.Consume(context.Background(), topic, group, func(_ context.Context, data []byte, id string) error {
		time.Sleep(time.Second)
		return nil
	}); err != nil {
		fmt.Printf("Finish example with error: %v", err)
	}
}
