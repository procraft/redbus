package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prokraft/redbus/internal/app/model"
	"gopkg.in/antage/eventsource.v1"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

type route struct {
	path    string
	handler http.HandlerFunc
}

func (a *AdminApi) RegisterHandlers(
	authMiddleware func(next http.Handler) http.Handler,
	m ...func(next http.Handler) http.Handler,
) context.CancelFunc {
	publicRoutes := []route{
		{path: "/health", handler: h(a.healthHandler, http.MethodGet)},
	}
	apiRoutes := []route{
		{path: "/dashboard/stat", handler: h(a.dashboardStatHandler)},
		{path: "/topic/stat", handler: h(a.topicStatHandler)},
		{path: "/repeat/stat", handler: h(a.repeatStatHandler)},
		{path: "/repeat/repeatTopicGroup", handler: h(a.repeatTopicGroupHandler)},
	}
	for _, r := range publicRoutes {
		http.Handle(r.path, middlewareChain(r.handler, m...))
	}
	baseBaseUrl := "/api"
	mWithAuth := []func(next http.Handler) http.Handler{authMiddleware}
	mWithAuth = append(mWithAuth, m...)
	for _, r := range apiRoutes {
		http.Handle(baseBaseUrl+r.path, middlewareChain(r.handler, mWithAuth...))
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
	http.Handle(baseBaseUrl+"/events", es)
	i := 0
	a.eventSource.Handler(func(event model.Event) {
		es.SendEventMessage(event.GetData(), event.GetName(), strconv.Itoa(i))
		i++
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
			sendErrorResponse(w, err, http.StatusInternalServerError)
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
