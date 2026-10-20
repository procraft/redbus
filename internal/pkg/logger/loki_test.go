package logger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type lokiServer struct {
	mu     sync.Mutex
	pushes []lokiPush
	auth   []string
}

func (l *lokiServer) handler(w http.ResponseWriter, r *http.Request) {
	var p lokiPush
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	user, pass, _ := r.BasicAuth()
	l.mu.Lock()
	l.pushes = append(l.pushes, p)
	l.auth = append(l.auth, user+":"+pass)
	l.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (l *lokiServer) lines() map[string][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	ret := map[string][]string{}
	for _, p := range l.pushes {
		for _, s := range p.Streams {
			key := s.Stream["app"] + "/" + s.Stream["l"]
			for _, v := range s.Values {
				ret[key] = append(ret[key], v[1])
			}
		}
	}
	return ret
}

func TestStartLokiPushesLinesWithAppAndLevelLabelsAndFlushesOnStop(t *testing.T) {
	srv := &lokiServer{}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer ts.Close()

	stop := StartLoki(LokiConfig{URL: ts.URL, Username: "u", Password: "p", App: "redbus", MinLevel: LevelInfo})
	Debug(App, "not pushed")
	Info(App, "hello %d\n", 1)
	Error(App, "broken")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stop(ctx)

	lines := srv.lines()
	require.Equal(t, []string{"[<none>] Push logs to Loki as app=redbus", "[<none>] hello 1"}, lines["redbus/INFO"])
	require.Equal(t, []string{"[<none>] broken"}, lines["redbus/ERROR"])
	require.NotContains(t, lines, "redbus/DEBUG")
	require.Equal(t, "u:p", srv.auth[0])

	// After stop nothing is queued any more.
	Info(App, "after stop")
	require.Len(t, srv.lines()["redbus/INFO"], 2)
}

func TestLokiSinkNeverBlocksWhenLokiIsUnreachable(t *testing.T) {
	blocked := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-blocked }))
	defer ts.Close()
	defer close(blocked)
	s := newLokiSink(LokiConfig{URL: ts.URL, App: "redbus"}, &http.Client{Timeout: 50 * time.Millisecond})
	go s.run()

	started := time.Now()
	for i := 0; i < lokiQueueSize*3; i++ {
		s.add(LevelInfo, "line")
	}
	require.Less(t, time.Since(started), time.Second, "adding lines must not wait for Loki")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.stop(ctx)
}

func TestStartLokiWithoutURLIsDisabled(t *testing.T) {
	stop := StartLoki(LokiConfig{})
	require.Nil(t, loki.Load())
	stop(context.Background())
}

func TestLokiPayloadAddsEnvLabelWhenSet(t *testing.T) {
	s := newLokiSink(LokiConfig{App: "redbus", Env: "prod"}, nil)
	p := s.payload([]lokiEntry{{LevelError, time.Unix(0, 1), "x"}})
	require.Equal(t, map[string]string{"app": "redbus", "l": "ERROR", "env": "prod"}, p.Streams[0].Stream)
}

func TestLokiPayloadGroupsByLevel(t *testing.T) {
	s := newLokiSink(LokiConfig{App: "redbus-admin"}, nil)
	ts := time.Unix(0, 42)
	p := s.payload([]lokiEntry{{LevelInfo, ts, "a"}, {LevelWarning, ts, "b"}, {LevelInfo, ts, "c"}})
	require.Len(t, p.Streams, 2)
	require.Equal(t, map[string]string{"app": "redbus-admin", "l": "INFO"}, p.Streams[0].Stream)
	require.Equal(t, [][2]string{{"42", "a"}, {"42", "c"}}, p.Streams[0].Values)
	require.Equal(t, "WARN", p.Streams[1].Stream["l"])
}
