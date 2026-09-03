package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/prokraft/redbus/internal/app/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/antage/eventsource.v1"
)

type route struct {
	path    string
	handler http.HandlerFunc
}

func (a *AdminApi) RegisterHandlers(
	mux *http.ServeMux,
	authMiddleware func(next http.Handler) http.Handler,
	m ...func(next http.Handler) http.Handler,
) context.CancelFunc {
	publicRoutes := []route{
		{path: "/health", handler: h(a.healthHandler, http.MethodGet)},
		{path: "/health/live", handler: h(a.liveHandler, http.MethodGet)},
		{path: "/health/ready", handler: h(a.healthHandler, http.MethodGet)},
	}
	apiRoutes := []route{
		{path: "/dashboard/stat", handler: h(a.dashboardStatHandler)},
		{path: "/topic/stat", handler: h(a.topicStatHandler)},
		{path: "/consumer/stat", handler: h(a.consumerStatHandler)},
		{path: "/repeat/stat", handler: h(a.repeatStatHandler)},
		{path: "/repeat/repeatTopicGroup", handler: h(a.repeatTopicGroupHandler)},
		{path: "/repeat/repeatTopicGroupSince", handler: h(a.repeatTopicGroupSinceHandler)},
		{path: "/repeat/repeatError", handler: h(a.repeatErrorHandler)},
	}
	for _, r := range publicRoutes {
		mux.Handle(r.path, middlewareChain(r.handler, m...))
	}
	baseBaseUrl := "/api"
	mWithAuth := []func(next http.Handler) http.Handler{authMiddleware}
	mWithAuth = append(mWithAuth, m...)
	for _, r := range apiRoutes {
		mux.Handle(baseBaseUrl+r.path, middlewareChain(r.handler, mWithAuth...))
	}

	es := eventsource.New(
		eventsource.DefaultSettings(),
		func(req *http.Request) [][]byte {
			return [][]byte{
				[]byte("X-Accel-Buffering: no"),
				[]byte("Access-Control-Allow-Origin: *"),
			}
		},
	)
	a.eventConsumersCount = es.ConsumersCount
	mux.Handle(baseBaseUrl+"/events", es)
	var eventID atomic.Uint64
	a.eventSource.Handler(func(event model.Event) {
		es.SendEventMessage(event.GetData(), event.GetName(), strconv.FormatUint(eventID.Add(1), 10))
	})

	return func() {
		es.Close()
	}
}

func h[REQ any, RESP any](fn func(ctx context.Context, req REQ) (*RESP, error), methods ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()

		if len(methods) == 0 {
			methods = []string{http.MethodPost, http.MethodOptions}
		}
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Methods", strings.Join(methods, ","))
		headers.Set("Access-Control-Allow-Headers", "*")

		// Если это предварительный запрос (OPTIONS), отвечаем только CORS-заголовками и завершаем обработку
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if !slices.Contains(methods, r.Method) {
			sendErrorResponse(w, fmt.Errorf("expected %v request, got %v", methods, r.Method), http.StatusMethodNotAllowed)
			return
		}

		headers.Set("Content-Type", "application/json")

		var req REQ
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				sendErrorResponse(w, err, http.StatusBadRequest)
				return
			}

			err = json.Unmarshal(body, &req)
			if err != nil {
				sendErrorResponse(w, err, http.StatusBadRequest)
				return
			}
		}

		resp, err := fn(r.Context(), req)
		if err != nil {
			sendErrorResponse(w, err, responseStatus(err))
			return
		}

		respJson, err := json.Marshal(resp)
		if err != nil {
			sendErrorResponse(w, err, http.StatusInternalServerError)
			return
		}

		headers.Set("Content-Length", strconv.Itoa(len(respJson)))
		_, err = w.Write(respJson)
		if err != nil {
			sendErrorResponse(w, err, http.StatusInternalServerError)
			return
		}
	}
}

func responseStatus(err error) int {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unavailable, codes.DeadlineExceeded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func sendErrorResponse(w http.ResponseWriter, respErr error, respCode int) {
	respJson, err := json.Marshal(errorResponse{Error: respErr.Error()})
	if err != nil {
		http.Error(w, fmt.Sprintf("%+v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(respJson)))
	w.WriteHeader(respCode)
	_, _ = w.Write(respJson)
}

type emptyRequest struct{}
type emptyResponse struct{}
type errorResponse struct {
	Error string `json:"error"`
}

func middlewareChain(h http.Handler, m ...func(next http.Handler) http.Handler) http.Handler {
	if len(m) == 0 {
		return h
	}
	if m[0] == nil {
		panic("middlewareChain: found nil middleware")
	}
	return m[0](middlewareChain(h, m[1:]...))
}
