package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestNew_ProducesValidJSONLinesWithLevelAndMessage(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	logger.Info("hello", "request_id", "abc-123")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output isn't valid JSON: %v (%s)", err, buf.String())
	}
	if line["msg"] != "hello" {
		t.Errorf("msg = %v, want %q", line["msg"], "hello")
	}
	if line["request_id"] != "abc-123" {
		t.Errorf("request_id = %v, want %q", line["request_id"], "abc-123")
	}
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want %q", line["level"], "INFO")
	}
}

func TestFromContext_ReturnsTheAttachedLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	ctx := WithLogger(context.Background(), logger)

	FromContext(ctx).Info("test")

	if buf.Len() == 0 {
		t.Error("FromContext did not return the logger attached via WithLogger -- nothing was written to it")
	}
}

func TestFromContext_FallsBackToSlogDefaultWhenNothingAttached(t *testing.T) {
	if got := FromContext(context.Background()); got == nil {
		t.Fatal("FromContext(no logger attached) = nil, want slog.Default()")
	}
}

func TestWithLogger_FieldsAccumulateAcrossNestedAttachments(t *testing.T) {
	var buf bytes.Buffer
	base := New(&buf)
	ctx := WithLogger(context.Background(), base.With("request_id", "req-1"))
	ctx = WithLogger(ctx, FromContext(ctx).With("session_id", "sess-1"))

	FromContext(ctx).Info("both fields present")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output isn't valid JSON: %v (%s)", err, buf.String())
	}
	if line["request_id"] != "req-1" || line["session_id"] != "sess-1" {
		t.Errorf("line = %v, want both request_id=req-1 and session_id=sess-1", line)
	}
}
