package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/prokraft/redbus/internal/app/controlapi"
	"github.com/prokraft/redbus/internal/app/grpcapi"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/app/repository"
	"github.com/prokraft/redbus/internal/app/service/connstore"
	"github.com/prokraft/redbus/internal/app/service/databus"
	"github.com/prokraft/redbus/internal/app/service/repeater"
	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/reqid"
	bgpkg "github.com/prokraft/redbus/internal/pkg/background"
	"github.com/prokraft/redbus/internal/pkg/db"
	dbmw "github.com/prokraft/redbus/internal/pkg/db/interceptor"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/prokraft/redbus/internal/pkg/kafka/producer"
	"github.com/prokraft/redbus/internal/pkg/kafka/provider"
	"github.com/prokraft/redbus/internal/pkg/logger"
	metricspkg "github.com/prokraft/redbus/internal/pkg/metrics"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type App struct {
	conf                  *config.Config
	dataBusService        *databus.DataBus
	repeaterService       *repeater.Repeater
	grpcServer            *grpc.Server
	controlServer         *grpc.Server
	dbClient              db.IClient
	grpcUnaryInterceptor  []grpc.UnaryServerInterceptor
	grpcStreamInterceptor []grpc.StreamServerInterceptor
	background            *bgpkg.Background
	metrics               *metricspkg.Metrics
	metricsServer         *http.Server
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
	eg.Go(a.getGrpcListener(egCtx, "public", a.conf.Grpc.ServerPort, a.grpcServer))
	eg.Go(a.getGrpcListener(egCtx, "control", a.conf.Control.ServerPort, a.controlServer))
	if a.conf.Metrics.ServerPort > 0 {
		eg.Go(a.getMetricsListener(egCtx))
	}
	for _, fn := range a.getBackgroundExecutorList(egCtx) {
		eg.Go(fn)
	}

	return eg.Wait()
}

func (a *App) initDeps(ctx context.Context) error {
	inits := []func(context.Context) error{
		a.initMetrics,
		a.initDb,
		a.initService,
		a.initBackground,
		a.initGrpcApi,
	}

	for _, fn := range inits {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) initMetrics(_ context.Context) error {
	a.metrics = metricspkg.New()
	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor, a.metrics.UnaryServerInterceptor())
	a.grpcStreamInterceptor = append(a.grpcStreamInterceptor, a.metrics.StreamServerInterceptor())
	mux := http.NewServeMux()
	mux.Handle("/metrics", a.metrics.Handler())
	a.metricsServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", a.conf.Metrics.ServerPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
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
	if pool, ok := a.dbClient.(metricspkg.DBPoolStatsProvider); ok {
		a.metrics.RegisterDBPool(pool)
	}

	dbFn := func(ctx context.Context) db.IClient {
		return a.dbClient
	}
	a.grpcUnaryInterceptor = append(a.grpcUnaryInterceptor, dbmw.UnaryServerInterceptor(dbFn))
	a.grpcStreamInterceptor = append(a.grpcStreamInterceptor, dbmw.StreamServerInterceptor(dbFn))
	return nil
}

func (a *App) initService(ctx context.Context) error {
	kafkaCredentials := credential.FromConf(a.conf.Kafka.Credentials)
	createProducerFn := func(ctx context.Context, topic model.TopicName) (model.IProducer, error) {
		options := []producer.Option{
			producer.WithLog(),
			producer.WithBalancer(&kafka.RoundRobin{}),
		}
		if a.conf.Kafka.CreateTopicIfNotExists {
			options = append(options, producer.WithCreateTopic(a.conf.Kafka.TopicNumPartitions, a.conf.Kafka.TopicReplicationFactor))
		}
		return producer.New(
			ctx,
			[]string{a.conf.Kafka.HostPort},
			kafkaCredentials,
			topic,
			options...,
		)
	}
	connStoreService := connstore.New(createProducerFn)
	repeaterService := repeater.New(
		a.conf.Repeat.DefaultStrategy,
		connStoreService,
		repository.New(),
		a.metrics,
	)
	kafkaProvider, err := provider.New(ctx, a.conf.Kafka.HostPort, kafkaCredentials)
	if err != nil {
		return err
	}
	a.dataBusService = databus.New(
		a.conf,
		connStoreService,
		repeaterService,
		kafkaProvider,
		a.metrics,
	)
	a.repeaterService = repeaterService
	return nil
}

func (a *App) initBackground(_ context.Context) error {
	a.background.Add("repeat", func(ctx context.Context) error {
		return a.repeaterService.Repeat(ctx)
	}, a.conf.Repeat.Interval.Duration)
	if a.conf.Metrics.ServerPort > 0 {
		a.background.Add("metrics_retry_records", func(ctx context.Context) error {
			allCount, failedCount, err := a.repeaterService.GetCount(ctx)
			if err != nil {
				return err
			}
			a.metrics.SetRetryRecords(allCount-failedCount, failedCount)
			return nil
		}, a.conf.Repeat.Interval.Duration)
	}
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
	a.grpcServer = a.newGrpcServer()
	pb.RegisterRedbusServiceServer(a.grpcServer, grpcapi.New(a.conf, a.dataBusService, a.repeaterService, a.metrics))
	registerHealthServer(a.grpcServer)

	a.controlServer = a.newGrpcServer()
	admincontrol.RegisterAdminControlServiceServer(
		a.controlServer,
		controlapi.New(a.dataBusService, a.repeaterService),
	)
	registerHealthServer(a.controlServer)

	return nil
}

func (a *App) getMetricsListener(ctx context.Context) func() error {
	return func() error {
		logger.Info(logger.App, "Start Prometheus metrics server on port %d", a.conf.Metrics.ServerPort)
		errCh := make(chan error, 1)
		go func() {
			errCh <- a.metricsServer.ListenAndServe()
		}()
		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("failed to serve Prometheus metrics: %w", err)
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := a.metricsServer.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("shutdown Prometheus metrics server: %w", err)
			}
			return ctx.Err()
		}
	}
}

func (a *App) newGrpcServer() *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(a.grpcUnaryInterceptor...)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(a.grpcStreamInterceptor...)),
	)
}

func registerHealthServer(server *grpc.Server) {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
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

func (a *App) getGrpcListener(ctx context.Context, name string, port int, server *grpc.Server) func() error {
	return func() error {
		logger.Info(logger.App, "Start %s GRPC server on port %d", name, port)
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return fmt.Errorf("failed to listen on %s GRPC port: %w", name, err)
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Serve(listener)
		}()
		select {
		case err := <-errCh:
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			return fmt.Errorf("failed to serve %s GRPC: %w", name, err)
		case <-ctx.Done():
			server.Stop()
			return ctx.Err()
		}
	}
}

func (a *App) getBackgroundExecutorList(ctx context.Context) []func() error {
	dbCtx := db.AddToContext(ctx, a.dbClient)
	return a.background.GetRunFnList(dbCtx)
}
