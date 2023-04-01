package app

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/sergiusd/redbus/internal/app/repeater"
	"github.com/sergiusd/redbus/internal/app/repeater/repository"
	"github.com/sergiusd/redbus/internal/pkg/db"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"

	dbmw "github.com/sergiusd/redbus/internal/pkg/db/interceptor"

	"github.com/sergiusd/redbus/api/golang/pb"
	"github.com/sergiusd/redbus/internal/app/config"
	"github.com/sergiusd/redbus/internal/app/databus"
	"github.com/sergiusd/redbus/internal/pkg/kafka/producer"
)

type App struct {
	conf                 *config.Config
	grpcServer           *grpc.Server
	dbClient             db.IClient
	grpcUnaryInterceptor []grpc.UnaryServerInterceptor
}

func New(ctx context.Context, conf *config.Config) (*App, error) {
	a := &App{conf: conf}
	if err := a.initDeps(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Run() error {
	return a.runGrpcApi()
}

func (a *App) initDeps(ctx context.Context) error {
	inits := []func(context.Context) error{
		a.initDb,
		//a.initService,
		//a.initApi,
		//a.initKafkaConsumer,
		//a.initBackground,
		a.initGrpcApi,
		//a.initHttpApi,
	}

	for _, fn := range inits {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initDb(ctx context.Context) error {
	var err error
	a.dbClient, err = db.New(
		ctx,
		a.conf.DB.Host,
		a.conf.DB.Port,
		a.conf.DB.User,
		a.conf.DB.Password,
		a.conf.DB.Name,
		db.WithPoolSize(a.conf.DB.PoolSize),
	)
	if err != nil {
		return err
	}

	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor,
		dbmw.NewDBInterceptor(func(ctx context.Context) db.IClient {
			return a.dbClient
		}),
	)
	//a.httpMiddleware = append(a.httpMiddleware,
	//	dbmw.NewDBServerMiddleware(func(ctx context.Context) db.IClient {
	//		return a.dbClient
	//	}),
	//)
	return nil
}

func (a *App) initGrpcApi(ctx context.Context) error {
	a.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(a.grpcUnaryInterceptor...)))
	createProducerFn := func(ctx context.Context, topic string) (databus.IProducer, error) {
		return producer.New(ctx, []string{a.conf.KafkaHostPort}, topic,
			producer.WithCreateTopic(a.conf.KafkaTopicNumPartitions, a.conf.KafkaTopicReplicationFactor),
			producer.WithLog(),
			producer.WithBalancer(&kafka.RoundRobin{}),
		)
	}
	pb.RegisterStreamServiceServer(a.grpcServer, databus.New(
		a.conf,
		createProducerFn,
		repeater.New(repository.New()),
	))

	return nil
}

func (a *App) runGrpcApi() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.conf.GrpcServerPort))
	if err != nil {
		return fmt.Errorf("Failed to listen: %w", err)
	}

	log.Printf("Start GRPC server on port %d\n", a.conf.GrpcServerPort)
	if err := a.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("Failed to serve: %w", err)
	}
	return nil
}
