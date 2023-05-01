package background

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sergiusd/redbus/internal/pkg/app/interceptor/reqid"
	"github.com/sergiusd/redbus/internal/pkg/logger"
)

type Job struct {
	fn       JobFn
	interval time.Duration
}

type JobFn = func(ctx context.Context) error

type Background struct {
	jobMap map[string]Job
}

func New() *Background {
	return &Background{
		jobMap: make(map[string]Job),
	}
}

func (bg *Background) Add(name string, fn JobFn, interval time.Duration) {
	bg.jobMap[name] = Job{fn: fn, interval: interval}
}

func (bg *Background) GetRunFnList(ctx context.Context) []func() error {
	ret := make([]func() error, 0, len(bg.jobMap))
	nameList := make([]string, 0, len(bg.jobMap))
	for name, job := range bg.jobMap {
		nameList = append(nameList, fmt.Sprintf("'%s', interval = %v", name, job.interval))
		m := sync.Mutex{}
		fn := func() error {
			ticker := time.Tick(job.interval)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker:
					if m.TryLock() {
						bg.exec(ctx, name, job.fn)
						m.Unlock()
					}
				}
			}
		}
		ret = append(ret, fn)
	}
	logger.Info(logger.App, "Start background tasks: %v", strings.Join(nameList, ", "))
	return ret
}

func (bg *Background) exec(ctx context.Context, name string, fn JobFn) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(logger.App, "Background '%s' recover: %v", name, r)
		}
	}()
	ctx = reqid.SetRequestId(ctx, "bg")
	if err := fn(ctx); err != nil {
		logger.Error(logger.App, "Background '%s' error: %v", name, err)
	}
}
