package adminapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/prokraft/redbus/internal/app/adminapi"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/admincontrol"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/log"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/recovery"
	"github.com/prokraft/redbus/internal/pkg/app/interceptor/reqid"
	"github.com/prokraft/redbus/internal/pkg/evtsrc"
	"github.com/prokraft/redbus/internal/pkg/logger"

	"golang.org/x/sync/errgroup"
)

const shutdownTimeout = 10 * time.Second

type App struct {
	conf                *config.Config
	client              *admincontrol.Client
	getStateSnapshot    func(context.Context) (model.Stat, error)
	eventConsumersCount func() int
	eventSource         *evtsrc.EventSource
	httpServer          *http.Server
	cancelSSE           context.CancelFunc
}

func New(conf *config.Config) (*App, error) {
	apiHost := strings.TrimSpace(conf.Admin.ApiHost)
	if apiHost == "" {
		return nil, errors.New("REDBUS_API_HOST is required")
	}

	requestTimeout := conf.Admin.RequestTimeout.Duration
	if requestTimeout <= 0 {
		requestTimeout = 5 * time.Second
	}
	client, err := admincontrol.New(conf.Admin.ControlAddress, requestTimeout)
	if err != nil {
		return nil, fmt.Errorf("create admin control client: %w", err)
	}

	eventSource := evtsrc.New()
	mux := http.NewServeMux()
	mux.Handle("/runtime-config.js", runtimeConfigHandler(apiHost))
	api := adminapi.New(client, eventSource)
	cancelSSE := api.RegisterHandlers(
		mux,
		adminapi.AuthMiddleware(conf.Admin.Token),
		log.ServerMiddleware(),
		reqid.ServerMiddleware("admin"),
		recovery.ServerMiddleware,
	)
	mux.Handle("/", spaHandler(conf.Admin.StaticDir))

	return &App{
		conf:                conf,
		client:              client,
		getStateSnapshot:    client.GetStateSnapshot,
		eventConsumersCount: api.EventConsumersCount,
		eventSource:         eventSource,
		cancelSSE:           cancelSSE,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%d", conf.Admin.ServerPort),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.cancelSSE()
	defer func() {
		if err := a.client.Close(); err != nil {
			logger.Error(logger.App, "Close admin control client: %v", err)
		}
	}()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(a.getHTTPListener(egCtx))
	eg.Go(a.getStatPoller(egCtx))
	return eg.Wait()
}

func (a *App) getHTTPListener(ctx context.Context) func() error {
	return func() error {
		logger.Info(logger.App, "Start Admin server on port %d", a.conf.Admin.ServerPort)
		errCh := make(chan error, 1)
		go func() {
			errCh <- a.httpServer.ListenAndServe()
		}()

		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("serve admin HTTP: %w", err)
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("shutdown admin HTTP: %w", err)
			}
			return ctx.Err()
		}
	}
}

func (a *App) getStatPoller(ctx context.Context) func() error {
	return func() error {
		interval := a.conf.Admin.PollInterval.Duration
		if interval <= 0 {
			interval = 5 * time.Second
		}

		var previous model.Stat
		var initialized bool
		poll := func() {
			if a.eventConsumersCount() == 0 {
				initialized = false
				return
			}

			stat, err := a.getStateSnapshot(ctx)
			if err != nil {
				logger.Error(logger.App, "Poll admin state: %v", err)
				return
			}
			if !initialized || stat.ConsumerCount != previous.ConsumerCount || stat.ConsumeTopicCount != previous.ConsumeTopicCount {
				a.eventSource.Publish(func() model.Event {
					return model.EventConsumers{
						ConsumerCount:     stat.ConsumerCount,
						ConsumeTopicCount: stat.ConsumeTopicCount,
					}
				})
			}
			if !initialized || stat.RepeatAllCount != previous.RepeatAllCount || stat.RepeatFailedCount != previous.RepeatFailedCount {
				a.eventSource.Publish(func() model.Event {
					return model.EventRepeater{
						AllCount:    stat.RepeatAllCount,
						FailedCount: stat.RepeatFailedCount,
					}
				})
			}
			previous = stat
			initialized = true
		}

		poll()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				poll()
			}
		}
	}
}

func spaHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		filePath := filepath.Join(staticDir, filepath.FromSlash(cleanPath))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
