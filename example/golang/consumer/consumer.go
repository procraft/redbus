package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prokraft/redbus/api/golang/consumer"
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
		consumer.WithServiceUnavailableTimeout(dataBusUnavailableTimeout),
	)
	if err := c.Consume(
		context.Background(),
		topic,
		group,
		func(_ context.Context, data []byte, id string) error {
			time.Sleep(time.Second)
			str := string(data)
			if strings.Contains(str, "error") {
				return fmt.Errorf("Some error")
			}
			if strings.Contains(str, "panic") {
				panic("I'm panic in consumer")
			}
			return nil
		},
		consumer.WithRepeatStrategyEven(3, 30),
		consumer.WithBatchSize(5),
	); err != nil {
		fmt.Printf("Finish example with error: %v", err)
	}
}
