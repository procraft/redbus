package main

import (
	"fmt"
	"github.com/sergiusd/redbus/api/golang/pb"
	"log"
	"net"

	"github.com/sergiusd/redbus/internal/app/config"
	"github.com/sergiusd/redbus/internal/app/databus"

	"google.golang.org/grpc"
)

func main() {
	conf := config.New()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.ServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterStreamServiceServer(s, databus.New(conf))

	log.Println("Start")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

}
