package gateway

import (
	"context"
	"errors"
	"testing"
)

func TestLifecycleHooks_Register(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	lh.Register(HookOnConnect, "monitor", func(_ context.Context, _ *LifecycleEvent) error {
		return nil
	}, 0)

	if !lh.HasHooks(HookOnConnect) {
		t.Error("expected on_connect hooks registered")
	}
	if lh.HookCount(HookOnConnect) != 1 {
		t.Errorf("expected 1 hook, got %d", lh.HookCount(HookOnConnect))
	}
}

func TestLifecycleHooks_PriorityOrder(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var order []string

	lh.Register(HookOnConnect, "low", func(_ context.Context, _ *LifecycleEvent) error {
		order = append(order, "low")
		return nil
	}, 10)

	lh.Register(HookOnConnect, "high", func(_ context.Context, _ *LifecycleEvent) error {
		order = append(order, "high")
		return nil
	}, 1)

	lh.Register(HookOnConnect, "mid", func(_ context.Context, _ *LifecycleEvent) error {
		order = append(order, "mid")
		return nil
	}, 5)

	lh.EmitConnect(context.Background(), PlatformTelegram)
	expected := []string{"high", "mid", "low"}
	if len(order) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestLifecycleHooks_EmitConnect(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var received *LifecycleEvent

	lh.Register(HookOnConnect, "test", func(_ context.Context, event *LifecycleEvent) error {
		received = event
		return nil
	}, 0)

	lh.EmitConnect(context.Background(), PlatformSlack)
	if received == nil {
		t.Fatal("hook not fired")
	}
	if received.Platform != PlatformSlack {
		t.Errorf("expected platform slack, got %s", received.Platform)
	}
	if received.Type != HookOnConnect {
		t.Errorf("expected type on_connect, got %s", received.Type)
	}
	if received.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestLifecycleHooks_EmitDisconnect(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var received *LifecycleEvent

	lh.Register(HookOnDisconnect, "test", func(_ context.Context, event *LifecycleEvent) error {
		received = event
		return nil
	}, 0)

	reason := errors.New("connection timeout")
	lh.EmitDisconnect(context.Background(), PlatformDiscord, reason)
	if received == nil {
		t.Fatal("hook not fired")
	}
	if received.Error != reason {
		t.Error("expected disconnect reason in event")
	}
}

func TestLifecycleHooks_EmitSessionStart(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var received *LifecycleEvent

	lh.Register(HookOnSessionStart, "test", func(_ context.Context, event *LifecycleEvent) error {
		received = event
		return nil
	}, 0)

	source := &SessionSource{Platform: PlatformTelegram, ChatID: "123", UserID: "user1"}
	lh.EmitSessionStart(context.Background(), PlatformTelegram, "session-key-1", source)
	if received == nil {
		t.Fatal("hook not fired")
	}
	if received.SessionKey != "session-key-1" {
		t.Errorf("expected session-key-1, got %s", received.SessionKey)
	}
	if received.Source.UserID != "user1" {
		t.Error("expected source.UserID = user1")
	}
}

func TestLifecycleHooks_EmitSessionEnd(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	fired := false

	lh.Register(HookOnSessionEnd, "test", func(_ context.Context, event *LifecycleEvent) error {
		fired = true
		return nil
	}, 0)

	lh.EmitSessionEnd(context.Background(), PlatformTelegram, "session-key-1")
	if !fired {
		t.Error("session end hook not fired")
	}
}

func TestLifecycleHooks_EmitMediaReceive(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var received *LifecycleEvent

	lh.Register(HookOnMediaReceive, "test", func(_ context.Context, event *LifecycleEvent) error {
		received = event
		return nil
	}, 0)

	lh.EmitMediaReceive(context.Background(), PlatformTelegram, MediaTypeImage, "/tmp/photo.jpg")
	if received == nil {
		t.Fatal("hook not fired")
	}
	if received.MediaType != MediaTypeImage {
		t.Errorf("expected image, got %s", received.MediaType)
	}
	if received.MediaPath != "/tmp/photo.jpg" {
		t.Errorf("expected /tmp/photo.jpg, got %s", received.MediaPath)
	}
}

func TestLifecycleHooks_EmitMediaSend(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	var received *LifecycleEvent

	lh.Register(HookOnMediaSend, "test", func(_ context.Context, event *LifecycleEvent) error {
		received = event
		return nil
	}, 0)

	lh.EmitMediaSend(context.Background(), PlatformSlack, MediaTypeDocument, "/tmp/report.pdf")
	if received == nil {
		t.Fatal("hook not fired")
	}
	if received.MediaType != MediaTypeDocument {
		t.Errorf("expected document, got %s", received.MediaType)
	}
}

func TestLifecycleHooks_ErrorPropagation(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	expectedErr := errors.New("hook failed")

	lh.Register(HookOnConnect, "failing", func(_ context.Context, _ *LifecycleEvent) error {
		return expectedErr
	}, 0)

	err := lh.EmitConnect(context.Background(), PlatformTelegram)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestLifecycleHooks_ContinuesAfterError(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	secondFired := false

	lh.Register(HookOnConnect, "failing", func(_ context.Context, _ *LifecycleEvent) error {
		return errors.New("first fails")
	}, 1)

	lh.Register(HookOnConnect, "succeeding", func(_ context.Context, _ *LifecycleEvent) error {
		secondFired = true
		return nil
	}, 2)

	lh.EmitConnect(context.Background(), PlatformTelegram)
	if !secondFired {
		t.Error("second hook should fire even after first fails")
	}
}

func TestLifecycleHooks_NoHooks(t *testing.T) {
	lh := NewLifecycleHooks(nil)
	err := lh.EmitConnect(context.Background(), PlatformTelegram)
	if err != nil {
		t.Error("should return nil when no hooks registered")
	}
	if lh.HasHooks(HookOnConnect) {
		t.Error("should report no hooks")
	}
}
