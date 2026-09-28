package service

import (
	"context"
	"testing"
)

func TestDeliverReplyEmpty(t *testing.T) {
	result := DeliverReply(context.Background(), nil, DeliveryRequest{Reply: "  "})
	if result.Status != DeliveryResultEmpty || len(result.Items) != 0 {
		t.Fatalf("unexpected empty delivery result: %#v", result)
	}
}

func TestDeliverReplyCancelledBeforeFirstItem(t *testing.T) {
	cancel := make(chan struct{})
	close(cancel)
	result := DeliverReply(context.Background(), nil, DeliveryRequest{Reply: "第一句$第二句", Cancel: cancel})
	if result.Status != DeliveryResultCancelled || result.FirstCommitted {
		t.Fatalf("unexpected cancelled delivery result: %#v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Status != DeliveryCancelled || result.Items[1].Status != DeliveryCancelled {
		t.Fatalf("expected all items cancelled: %#v", result.Items)
	}
}
