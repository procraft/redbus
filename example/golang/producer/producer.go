package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sergiusd/redbus/api/golang/pb"
	"log"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
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

	// bus client
	busConn, err := grpc.Dial(
		fmt.Sprintf(":%v", dataBusServerPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("can not connect with databus %v", err)
	}
	busClient := pb.NewStreamServiceClient(busConn)

	resp, err := busClient.Produce(context.Background(), &pb.ProduceRequest{
		Topic:   topic,
		Key:     key,
		Message: message,
	})
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Response: %v\n", resp)
}
