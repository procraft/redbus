package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Loki push defaults. The service pushes its own logs: the cluster has no log collector agent.
const (
	lokiQueueSize     = 10000
	lokiBatchSize     = 500
	lokiFlushInterval = 2 * time.Second
	lokiSendTimeout   = 10 * time.Second
	lokiErrorInterval = time.Minute
)

// LokiConfig enables pushing log lines to Loki; an empty URL disables it.
type LokiConfig struct {
	URL      string // push endpoint, e.g. https://loki.example/loki/api/v1/push
	Username string // basic auth, optional
	Password string
	App      string // value of the "app" stream label
	Env      string // value of the "env" stream label; empty means no label
	MinLevel Level  // lines below it are not pushed; empty means LevelDebug
}

type lokiEntry struct {
	level Level
	ts    time.Time
	line  string
}

// lokiSink batches lines in a goroutine. Log never blocks on it: a full queue drops the line and
// counts it, so an unreachable Loki slows nothing down.
type lokiSink struct {
	conf    LokiConfig
	client  *http.Client
	queue   chan lokiEntry
	flushCh chan chan struct{}
	done    chan struct{}
	dropped atomic.Int64

	lastErrorAt time.Time // touched only by the worker
	stopOnce    sync.Once
}

var loki atomic.Pointer[lokiSink]

// StartLoki starts pushing log lines to Loki and returns the function that flushes the queue and
// stops the sink; ctx bounds that final flush. With an empty URL it does nothing.
func StartLoki(conf LokiConfig) (stop func(ctx context.Context)) {
	if conf.URL == "" {
		return func(context.Context) {}
	}
	s := newLokiSink(conf, &http.Client{Timeout: lokiSendTimeout})
	loki.Store(s)
	go s.run()
	Info(App, "Push logs to Loki as app=%s", conf.App)
	return func(ctx context.Context) {
		loki.CompareAndSwap(s, nil)
		s.stop(ctx)
	}
}

func newLokiSink(conf LokiConfig, client *http.Client) *lokiSink {
	if conf.MinLevel == "" {
		conf.MinLevel = LevelDebug
	}
	return &lokiSink{
		conf:    conf,
		client:  client,
		queue:   make(chan lokiEntry, lokiQueueSize),
		flushCh: make(chan chan struct{}),
		done:    make(chan struct{}),
	}
}

// flushLoki pushes everything queued so far, waiting at most timeout; used before os.Exit.
func flushLoki(timeout time.Duration) {
	if s := loki.Load(); s != nil {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		s.flush(ctx)
	}
}

func sendToLoki(level Level, line string) {
	if s := loki.Load(); s != nil {
		s.add(level, line)
	}
}

func (s *lokiSink) add(level Level, line string) {
	if levelRank[level] < levelRank[s.conf.MinLevel] {
		return
	}
	select {
	case s.queue <- lokiEntry{level: level, ts: time.Now(), line: line}:
	default:
		s.dropped.Add(1)
	}
}

func (s *lokiSink) run() {
	defer close(s.done)
	ticker := time.NewTicker(lokiFlushInterval)
	defer ticker.Stop()
	batch := make([]lokiEntry, 0, lokiBatchSize)
	push := func() {
		if dropped := s.dropped.Swap(0); dropped > 0 {
			batch = append(batch, lokiEntry{
				level: LevelWarning,
				ts:    time.Now(),
				line:  fmt.Sprintf("Loki queue overflow: %d log lines dropped", dropped),
			})
		}
		if len(batch) == 0 {
			return
		}
		if err := s.send(batch); err != nil {
			s.reportError(err, len(batch))
		}
		batch = batch[:0]
	}
	drain := func() {
		for {
			select {
			case e := <-s.queue:
				batch = append(batch, e)
				if len(batch) >= lokiBatchSize {
					push()
				}
			default:
				push()
				return
			}
		}
	}
	for {
		select {
		case e := <-s.queue:
			batch = append(batch, e)
			if len(batch) >= lokiBatchSize {
				push()
			}
		case <-ticker.C:
			push()
		case ack, ok := <-s.flushCh:
			if !ok {
				drain()
				return
			}
			drain()
			close(ack)
		}
	}
}

// flush asks the worker to push the queue and waits for it or for ctx.
func (s *lokiSink) flush(ctx context.Context) {
	ack := make(chan struct{})
	select {
	case s.flushCh <- ack:
	case <-s.done:
		return
	case <-ctx.Done():
		return
	}
	select {
	case <-ack:
	case <-ctx.Done():
	}
}

func (s *lokiSink) stop(ctx context.Context) {
	s.stopOnce.Do(func() {
		close(s.flushCh)
		select {
		case <-s.done:
		case <-ctx.Done():
		}
	})
}

// reportError writes to the standard logger only (never back into the sink), once a minute.
func (s *lokiSink) reportError(err error, lines int) {
	now := time.Now()
	if now.Sub(s.lastErrorAt) < lokiErrorInterval {
		return
	}
	s.lastErrorAt = now
	log.Printf("[%s] Loki push failed, %d log lines lost: %v", LevelError, lines, err)
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

// payload groups the batch into one stream per level. Labels: app and l (DEBUG, INFO, WARN, ERROR,
// FATAL) as in the other Go services, plus env (prod, stage) as in the Scala services when set.
func (s *lokiSink) payload(batch []lokiEntry) lokiPush {
	var streams []lokiStream
	index := map[Level]int{}
	for _, e := range batch {
		i, ok := index[e.level]
		if !ok {
			i = len(streams)
			index[e.level] = i
			labels := map[string]string{"app": s.conf.App, "l": lokiLevel(e.level)}
			if s.conf.Env != "" {
				labels["env"] = s.conf.Env
			}
			streams = append(streams, lokiStream{Stream: labels})
		}
		streams[i].Values = append(streams[i].Values, [2]string{strconv.FormatInt(e.ts.UnixNano(), 10), e.line})
	}
	return lokiPush{Streams: streams}
}

func (s *lokiSink) send(batch []lokiEntry) error {
	body, err := json.Marshal(s.payload(batch))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, s.conf.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.conf.Username != "" || s.conf.Password != "" {
		req.SetBasicAuth(s.conf.Username, s.conf.Password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("loki answered %s", resp.Status)
	}
	return nil
}

var levelRank = map[Level]int{LevelDebug: 0, LevelInfo: 1, LevelWarning: 2, LevelError: 3, LevelFatal: 4}

func lokiLevel(level Level) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel maps a config value (debug, info, warning|warn, error) to a Level; empty is debug.
func ParseLevel(v string) (Level, error) {
	switch v {
	case "", "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warning", "warn":
		return LevelWarning, nil
	case "error":
		return LevelError, nil
	default:
		return "", fmt.Errorf("unknown log level %q", v)
	}
}
