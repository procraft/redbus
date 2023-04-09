package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sergiusd/redbus/internal/app/model"
	"github.com/sergiusd/redbus/internal/pkg/logger"

	"github.com/sergiusd/redbus/internal/app/repeater"
	"github.com/sergiusd/redbus/internal/app/repeater/repository"
	"github.com/sergiusd/redbus/internal/pkg/db"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"

	dbmw "github.com/sergiusd/redbus/internal/pkg/db/interceptor"

	"golang.org/x/sync/errgroup"

	"github.com/sergiusd/redbus/api/golang/pb"
	"github.com/sergiusd/redbus/internal/app/config"
	"github.com/sergiusd/redbus/internal/app/databus"
	"github.com/sergiusd/redbus/internal/pkg/kafka/producer"
)

type App struct {
	conf                 *config.Config
	dataBusService       *databus.DataBus
	grpcServer           *grpc.Server
	dbClient             db.IClient
	grpcUnaryInterceptor []grpc.UnaryServerInterceptor
	background           map[string]backgroundFn
}

type backgroundFn = func(ctx context.Context) error

func New(ctx context.Context, conf *config.Config) (*App, error) {
	a := &App{
		conf:       conf,
		background: make(map[string]backgroundFn),
	}
	if err := a.initDeps(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(a.runTerminateWatcher(egCtx, cancel))
	eg.Go(a.runGrpcApi(egCtx))
	eg.Go(a.runBackground(egCtx, time.Minute))

	return eg.Wait()
}

func (a *App) initDeps(ctx context.Context) error {
	inits := []func(context.Context) error{
		a.initDb,
		a.initService,
		//a.initApi,
		//a.initKafkaConsumer,
		a.initBackground,
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

func (a *App) initService(_ context.Context) error {
	createProducerFn := func(ctx context.Context, topic string) (model.IProducer, error) {
		return producer.New(ctx, []string{a.conf.KafkaHostPort}, topic,
			producer.WithCreateTopic(a.conf.KafkaTopicNumPartitions, a.conf.KafkaTopicReplicationFactor),
			producer.WithLog(),
			producer.WithBalancer(&kafka.RoundRobin{}),
		)
	}
	a.dataBusService = databus.New(
		a.conf,
		createProducerFn,
		repeater.New(repository.New()),
	)
	return nil
}

func (a *App) initBackground(_ context.Context) error {
	a.background["repeater"] = func(ctx context.Context) error {
		return a.dataBusService.Repeat(ctx)
	}
	return nil
}

func (a *App) initGrpcApi(_ context.Context) error {
	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor, NewRequestIdInterceptor())
	a.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(a.grpcUnaryInterceptor...)))
	pb.RegisterRedbusServiceServer(a.grpcServer, a.dataBusService)

	return nil
}

func (a *App) runTerminateWatcher(ctx context.Context, cancel context.CancelFunc) func() error {
	return func() error {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
		select {
		case <-signalCh:
			logger.Info(logger.App, "Catch terminate signal...")
			cancel()
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}
}

func (a *App) runGrpcApi(ctx context.Context) func() error {
	return func() error {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.conf.GrpcServerPort))
		if err != nil {
			return fmt.Errorf("Failed to listen: %w", err)
		}

		logger.Info(logger.App, "Start GRPC server on port %d", a.conf.GrpcServerPort)
		errCh := make(chan error)
		go func() {
			err := a.grpcServer.Serve(listener)
			errCh <- fmt.Errorf("Failed to serve: %w", err)
		}()
		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *App) runBackground(ctx context.Context, tickInterval time.Duration) func() error {
	return func() error {
		ticker := time.Tick(tickInterval)
		nameList := make([]string, 0, len(a.background))
		for name := range a.background {
			nameList = append(nameList, "'"+name+"'")
		}
		logger.Info(logger.App, "Start background tasks: %v, interval %v", strings.Join(nameList, ", "), tickInterval)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker:
				for name, fn := range a.background {
					ctx := SetRequestId(ctx, "bg")
					if err := fn(ctx); err != nil {
						logger.Error(logger.App, "Background '%s' error: %v", name, err)
					}
				}
			}
		}
	}
}
