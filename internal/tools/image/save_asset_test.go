package image

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"project-yume/internal/model"
)

type saveAssetTestTurn struct {
	parts []model.MessagePart
}

func (t saveAssetTestTurn) RequestID() string        { return "req-1" }
func (t saveAssetTestTurn) SessionID() string        { return "private:42" }
func (t saveAssetTestTurn) UserID() int64            { return 42 }
func (t saveAssetTestTurn) GroupID() int64           { return 0 }
func (t saveAssetTestTurn) ChatType() int            { return 1 }
func (t saveAssetTestTurn) Message() string          { return "保存这张图片" }
func (t saveAssetTestTurn) ReferenceTime() time.Time { return time.Unix(1, 0) }
func (t saveAssetTestTurn) Trigger() string          { return "message" }
func (t saveAssetTestTurn) Actor() string            { return "user" }
func (t saveAssetTestTurn) Parts() []model.MessagePart {
	return append([]model.MessagePart(nil), t.parts...)
}

func TestSaveAssetToolRequiresCurrentImageOrSourceURL(t *testing.T) {
	_, err := NewSaveAssetTool().Execute(
		context.Background(),
		saveAssetTestTurn{},
		json.RawMessage(`{"asset_id":"cat"}`),
	)
	if err == nil || !strings.Contains(err.Error(), "current turn has 0 images") {
		t.Fatalf("expected missing current image error, got %v", err)
	}
}

func TestSaveAssetToolUsesSelectedCurrentImage(t *testing.T) {
	tool := NewSaveAssetTool()
	if tool.Name() != "save_image_asset" {
		t.Fatalf("unexpected tool name: %s", tool.Name())
	}
	if tool.ReadOnly() {
		t.Fatal("save image tool must be write-enabled")
	}

	property, ok := tool.Schema().Properties["image_index"]
	if !ok || property.Type != "integer" {
		t.Fatal("expected image_index integer property")
	}
	if len(tool.Schema().Required) != 1 || tool.Schema().Required[0] != "asset_id" {
		t.Fatalf("unexpected required fields: %#v", tool.Schema().Required)
	}
}
