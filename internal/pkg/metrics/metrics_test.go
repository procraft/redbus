package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerExposesDomainAndRuntimeMetrics(t *testing.T) {
	m := New()
	m.ObserveProduce("orders", "success", 20*time.Millisecond)
	m.ObserveConsumed("orders", "billing", "success", 4)
	m.ObserveConsumerBatch("orders", "billing", 4, 100*time.Millisecond)
	m.ObserveRetryEnqueued("orders", "billing", "success")
	m.SetRetryRecords(3, 1)

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()

	if recorder.Code != 200 {
		t.Fatalf("unexpected response status: %d", recorder.Code)
	}
	for _, expected := range []string{
		`redbus_produced_messages_total{result="success",topic="orders"} 1`,
		`redbus_consumed_messages_total{group="billing",result="success",topic="orders"} 4`,
		`redbus_retry_enqueued_total{group="billing",result="success",topic="orders"} 1`,
		`redbus_retry_records{state="pending"} 3`,
		`redbus_retry_records{state="exhausted"} 1`,
		`go_goroutines`,
		`process_cpu_seconds_total`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics response does not contain %q", expected)
		}
	}
}

func TestLateStateChangeDoesNotRestoreRemovedConsumer(t *testing.T) {
	m := New()
	m.AddConsumer("orders", "billing", "worker-1", "connecting")
	m.ChangeConsumerState("orders", "billing", "worker-1", "connected")
	m.RemoveConsumer("orders", "billing", "worker-1")
	m.ChangeConsumerState("orders", "billing", "worker-1", "reconnecting")

	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()

	if strings.Contains(body, `redbus_active_consumers{group="billing",state="reconnecting",topic="orders"}`) {
		t.Fatal("late state transition restored a removed consumer")
	}
	if !strings.Contains(body, `redbus_active_consumers{group="billing",state="connected",topic="orders"} 0`) {
		t.Fatal("removed consumer gauge was not decremented")
	}
}

func TestSplitFullMethod(t *testing.T) {
	service, method := splitFullMethod("/redbus.RedbusService/Produce")
	if service != "redbus.RedbusService" || method != "Produce" {
		t.Fatalf("unexpected split: %q %q", service, method)
	}
}

func BenchmarkObserveConsumed(b *testing.B) {
	m := New()
	b.ReportAllocs()
	for b.Loop() {
		m.ObserveConsumed("orders", "billing", "success", 32)
	}
}

func BenchmarkObserveConsumerBatch(b *testing.B) {
	m := New()
	b.ReportAllocs()
	for b.Loop() {
		m.ObserveConsumerBatch("orders", "billing", 32, 100*time.Millisecond)
	}
}
