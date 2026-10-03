package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/procraft/redbus/internal/pkg/admincontrol"
)

type response struct {
	SinceUnixMs int64          `json:"sinceUnixMs"`
	UntilUnixMs int64          `json:"untilUnixMs"`
	List        []responseItem `json:"list"`
	// Queue is a snapshot of retries that are still waiting, taken when the command runs. It lets
	// the triage tell a queue or a consumer deferral apart from failures in the window.
	Queue []queueItem `json:"queue"`
}

// queueItem describes the current retry queue of one topic/group. PendingCount waits after an
// ordinary failure, DeferredCount was postponed by the consumer (retryLater), FailedCount is the
// all-time number of exhausted retries, not limited to the triage window.
type queueItem struct {
	Topic              string `json:"topic"`
	Group              string `json:"group"`
	PendingCount       int    `json:"pendingCount"`
	DeferredCount      int    `json:"deferredCount"`
	FailedCount        int    `json:"failedCount"`
	LastError          string `json:"lastError"`
	LastDeferredReason string `json:"lastDeferredReason"`
}

type responseItem struct {
	Topic       string          `json:"topic"`
	Group       string          `json:"group"`
	FailedCount int             `json:"failedCount"`
	Errors      []responseError `json:"errors"`
}

type responseError struct {
	Error         string `json:"error"`
	Sample        string `json:"sample"`
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
	// A bus without deferral support reports zero deferred retries; they then count as pending.
	retryStats, err := client.GetRetryStats(context.Background())
	if err != nil {
		return fmt.Errorf("get retry stats: %w", err)
	}
	result := toResponse(stat)
	result.Queue = toQueue(retryStats, *topic, *group)
	return json.NewEncoder(os.Stdout).Encode(result)
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
		Queue:       make([]queueItem, 0),
	}
	for _, item := range stat.List {
		errors := make([]responseError, 0, len(item.Errors))
		for _, itemError := range item.Errors {
			errors = append(errors, responseError{
				Error:         itemError.Error,
				Sample:        itemError.Sample,
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

// toQueue keeps the topic/groups matching the exact filters that still have waiting retries,
// largest queue first.
func toQueue(stat model.RepeatStat, topic, group string) []queueItem {
	result := make([]queueItem, 0)
	for _, item := range stat {
		if (topic != "" && item.Topic != topic) || (group != "" && item.Group != group) {
			continue
		}
		pending := max(item.AllCount-item.FailedCount-item.DeferredCount, 0)
		if pending+item.DeferredCount <= 0 {
			continue
		}
		result = append(result, queueItem{
			Topic:              item.Topic,
			Group:              item.Group,
			PendingCount:       pending,
			DeferredCount:      item.DeferredCount,
			FailedCount:        item.FailedCount,
			LastError:          item.LastError,
			LastDeferredReason: item.LastDeferredReason,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i].PendingCount+result[i].DeferredCount, result[j].PendingCount+result[j].DeferredCount
		if left != right {
			return left > right
		}
		if result[i].Topic != result[j].Topic {
			return result[i].Topic < result[j].Topic
		}
		return result[i].Group < result[j].Group
	})
	return result
}
