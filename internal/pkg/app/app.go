package app

import (
	"context"
	"fmt"
	"github.com/prokraft/redbus/internal/app/adminapi"
	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/log"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/recovery"
	"github.com/prokraft/redbus/internal/pkg/evtsrc"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/grpcapi"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/app/repository"
	"github.com/prokraft/redbus/internal/app/service/connstore"
	"github.com/prokraft/redbus/internal/app/service/databus"
	"github.com/prokraft/redbus/internal/app/service/repeater"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/reqid"
	bgpkg "github.com/prokraft/redbus/internal/pkg/background"
	"github.com/prokraft/redbus/internal/pkg/db"
	dbmw "github.com/prokraft/redbus/internal/pkg/db/interceptor"
	"github.com/prokraft/redbus/internal/pkg/kafka/producer"
	"github.com/prokraft/redbus/internal/pkg/logger"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type App struct {
	conf                  *config.Config
	eventSource           *evtsrc.EventSource
	dataBusService        *databus.DataBus
	repeaterService       *repeater.Repeater
	grpcServer            *grpc.Server
	dbClient              db.IClient
	grpcUnaryInterceptor  []grpc.UnaryServerInterceptor
	grpcStreamInterceptor []grpc.StreamServerInterceptor
	adminHttpMiddleware   []func(next http.Handler) http.Handler
	background            *bgpkg.Background
}

func New(ctx context.Context, conf *config.Config) (*App, error) {
	a := &App{
		conf:       conf,
		background: bgpkg.New(),
	}
	if err := a.initDeps(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(a.getTerminateWatcher(egCtx, cancel))
	eg.Go(a.getGrpcApiListener(egCtx))
	eg.Go(a.getAdminListener(egCtx))
	for _, fn := range a.getBackgroundExecutorList(egCtx) {
		eg.Go(fn)
	}

	return eg.Wait()
}

func (a *App) initDeps(ctx context.Context) error {
	inits := []func(context.Context) error{
		a.initDb,
		a.initService,
		a.initBackground,
		a.initGrpcApi,
		a.initAdminApi,
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

	dbFn := func(ctx context.Context) db.IClient {
		return a.dbClient
	}
	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor, dbmw.UnaryServerInterceptor(dbFn))
	a.grpcStreamInterceptor = append(a.grpcStreamInterceptor, dbmw.StreamServerInterceptor(dbFn))
	a.adminHttpMiddleware = append(a.adminHttpMiddleware, dbmw.ServerMiddleware(dbFn))
	return nil
}

func (a *App) initService(_ context.Context) error {
	a.eventSource = evtsrc.New()
	createProducerFn := func(ctx context.Context, topic string) (model.IProducer, error) {
		return producer.New(
			ctx,
			[]string{a.conf.Kafka.HostPort},
			credential.FromConf(a.conf.Kafka.Credentials),
			topic,
			producer.WithCreateTopic(a.conf.Kafka.TopicNumPartitions, a.conf.Kafka.TopicReplicationFactor),
			producer.WithLog(),
			producer.WithBalancer(&kafka.RoundRobin{}),
		)
	}
	connStoreService := connstore.New(createProducerFn, a.eventSource)
	repeaterService := repeater.New(
		a.conf.Repeat.DefaultStrategy,
		connStoreService,
		repository.New(),
		a.eventSource,
	)
	a.dataBusService = databus.New(
		a.conf,
		connStoreService,
		repeaterService,
	)
	a.repeaterService = repeaterService
	return nil
}

func (a *App) initBackground(_ context.Context) error {
	a.background.Add("repeat", func(ctx context.Context) error {
		return a.repeaterService.Repeat(ctx)
	}, a.conf.Repeat.Interval.Duration)
	return nil
}

func (a *App) initGrpcApi(_ context.Context) error {
	recoveryFn := grpc_recovery.WithRecoveryHandler(func(data interface{}) (err error) {
		logger.Error(logger.App, "Recovery: %+v", data)
		return nil
	})
	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor,
		reqid.UnaryServerInterceptor(),
		grpc_ctxtags.UnaryServerInterceptor(),
		grpc_recovery.UnaryServerInterceptor(recoveryFn),
	)
	a.grpcStreamInterceptor = append(a.grpcStreamInterceptor,
		reqid.StreamServerInterceptor(),
		grpc_ctxtags.StreamServerInterceptor(),
		grpc_recovery.StreamServerInterceptor(recoveryFn),
	)
	a.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(a.grpcUnaryInterceptor...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(a.grpcStreamInterceptor...)),
	)
	pb.RegisterRedbusServiceServer(a.grpcServer, grpcapi.New(a.conf, a.dataBusService, a.repeaterService))

	return nil
}

func (a *App) initAdminApi(_ context.Context) error {
	return nil
}

func (a *App) getTerminateWatcher(ctx context.Context, cancel context.CancelFunc) func() error {
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

func (a *App) getGrpcApiListener(ctx context.Context) func() error {
	return func() error {
		logger.Info(logger.App, "Start GRPC server on port %d", a.conf.Grpc.ServerPort)
		errCh := make(chan error)
		go func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.conf.Grpc.ServerPort))
			if err != nil {
				errCh <- fmt.Errorf("Failed to listen: %w", err)
				return
			}
			err = a.grpcServer.Serve(listener)
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

func (a *App) getAdminListener(ctx context.Context) func() error {
	return func() error {
		logger.Info(logger.App, "Start Admin server on port %d", a.conf.Admin.ServerPort)
		errCh := make(chan error)
		adminApi := adminapi.New(a.dataBusService, a.repeaterService, a.eventSource)

		a.adminHttpMiddleware = append(a.adminHttpMiddleware,
			log.ServerMiddleware(),
			reqid.ServerMiddleware("admin"),
			recovery.ServerMiddleware,
		)
		cancel := adminApi.RegisterHandlers(a.adminHttpMiddleware...)
		defer cancel()
		http.Handle("/", http.FileServer(http.Dir("./web/admin/dist")))

		go func() {
			err := http.ListenAndServe(fmt.Sprintf(":%d", a.conf.Admin.ServerPort), nil)
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

func (a *App) getBackgroundExecutorList(ctx context.Context) []func() error {
	dbCtx := db.AddToContext(ctx, a.dbClient)
	return a.background.GetRunFnList(dbCtx)
}
