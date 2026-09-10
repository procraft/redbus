package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/admincontrol"
)

type response struct {
	SinceUnixMs int64          `json:"sinceUnixMs"`
	UntilUnixMs int64          `json:"untilUnixMs"`
	List        []responseItem `json:"list"`
}

type responseItem struct {
	Topic       string          `json:"topic"`
	Group       string          `json:"group"`
	FailedCount int             `json:"failedCount"`
	Errors      []responseError `json:"errors"`
}

type responseError struct {
	Error         string `json:"error"`
	FailedCount   int    `json:"failedCount"`
	FirstFailedAt string `json:"firstFailedAt"`
	LastFailedAt  string `json:"lastFailedAt"`
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "redbus triage: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	address := flag.String("address", "", "loopback address of the RED Bus control gRPC tunnel")
	sinceUnixMs := flag.Int64("since-unix-ms", 0, "inclusive beginning of the failure window")
	untilUnixMs := flag.Int64("until-unix-ms", 0, "exclusive end of the failure window")
	topic := flag.String("topic", "", "optional exact topic filter")
	group := flag.String("group", "", "optional exact consumer group filter")
	flag.Parse()

	if err := validateLoopbackAddress(*address); err != nil {
		return err
	}
	if *sinceUnixMs <= 0 || *untilUnixMs <= *sinceUnixMs {
		return errors.New("invalid triage window")
	}

	client, err := admincontrol.New(*address, 30*time.Second)
	if err != nil {
		return fmt.Errorf("connect control API: %w", err)
	}
	defer client.Close() //nolint:errcheck -- the request result is already complete

	stat, err := client.GetRetryTriage(
		context.Background(),
		time.UnixMilli(*sinceUnixMs),
		time.UnixMilli(*untilUnixMs),
		*topic,
		*group,
	)
	if err != nil {
		return fmt.Errorf("get retry triage: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(toResponse(stat))
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("control address must be host:port: %w", err)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("control address must point to a local tunnel")
	}
	return nil
}

func toResponse(stat model.RepeatTriageStat) response {
	result := response{
		SinceUnixMs: stat.Since.UnixMilli(),
		UntilUnixMs: stat.Until.UnixMilli(),
		List:        make([]responseItem, 0, len(stat.List)),
	}
	for _, item := range stat.List {
		errors := make([]responseError, 0, len(item.Errors))
		for _, itemError := range item.Errors {
			errors = append(errors, responseError{
				Error:         itemError.Error,
				FailedCount:   itemError.FailedCount,
				FirstFailedAt: itemError.FirstFailedAt.UTC().Format(time.RFC3339Nano),
				LastFailedAt:  itemError.LastFailedAt.UTC().Format(time.RFC3339Nano),
			})
		}
		result.List = append(result.List, responseItem{
			Topic:       item.Topic,
			Group:       item.Group,
			FailedCount: item.FailedCount,
			Errors:      errors,
		})
	}
	return result
}
