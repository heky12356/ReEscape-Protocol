package app

import (
	"context"
	"testing"
	"time"

	"project-yume/internal/eventlog"
	"project-yume/internal/service"
	"project-yume/internal/storage"
)

func TestRuntimeDeliveryEventsPreserveAggregateProjection(t *testing.T) {
	persistence, err := storage.OpenPersistence(storage.PersistenceOptions{Backend: "sqlite", DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer persistence.Close()
	ctx := context.Background()
	store := eventlog.NewSQLiteEventStore(persistence.Database())
	at := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	for _, outcome := range []string{service.DeliveryResultDelivered, service.DeliveryResultPartial, service.DeliveryResultFailed, service.DeliveryResultCancelled} {
		t.Run(outcome, func(t *testing.T) {
			result := service.DeliveryResult{
				DeliveryID: outcome, TurnID: "turn-" + outcome, IntentID: "intent-" + outcome,
				SessionID: "private:42", UserID: 42, Status: outcome, StartedAt: at,
				FinishedAt: at.Add(time.Second),
				Items:      []service.DeliveryItemResult{{Status: service.DeliveryDelivered, MessageID: 9007199254740993, DeliveredAt: at}},
			}
			// Use the actual runtime producer, including its final Intent event.
			if err := store.Append(ctx, deliveryEvents(result, 42)...); err != nil {
				t.Fatal(err)
			}
			record, err := storage.NewSQLiteQueryStore(persistence.Database()).Get(ctx, "deliveries", outcome)
			if err != nil || record.Status != outcome || record.IntentID != result.IntentID || record.SessionID != result.SessionID || record.TurnID != result.TurnID {
				t.Fatalf("runtime outcome %s projected as %+v: %v", outcome, record, err)
			}
		})
	}
}
