package main

import (
	"context"
	"errors"
	"konsulin-service/internal/app/services/core/webhook"
	"testing"
)

// fakeForwarderTarget implements SetWebhookForwarder, capturing the wired
// forwarder so tests can invoke it directly.
type fakeForwarderTarget struct {
	fn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)
}

func (t *fakeForwarderTarget) SetWebhookForwarder(fn func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error)) {
	t.fn = fn
}

// TestWireWebhookForwarder_SetsForwarder verifies a target implementing
// SetWebhookForwarder gets wired to the synchronous webhook forwarder and that
// the wired function maps the usecase output (StatusCode, Body) into the
// (int, []byte) pair expected by in-process callers.
func TestWireWebhookForwarder_SetsForwarder(t *testing.T) {
	target := &fakeForwarderTarget{}
	forward := func(_ context.Context, _, _ string, _ []byte, _ string) (*webhook.HandleSynchronousWebhookServiceOutput, error) {
		return &webhook.HandleSynchronousWebhookServiceOutput{StatusCode: 201, Body: []byte("created")}, nil
	}
	wireWebhookForwarder(target, forward)

	if target.fn == nil {
		t.Fatal("expected forwarder to be wired")
	}
	code, body, err := target.fn(context.Background(), "svc", "POST", []byte("{}"), "application/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 201 || string(body) != "created" {
		t.Fatalf("got (%d, %q), want (201, %q)", code, body, "created")
	}
}

// TestWireWebhookForwarder_PropagatesError verifies the wired forwarder
// surfaces the underlying error without a partial result.
func TestWireWebhookForwarder_PropagatesError(t *testing.T) {
	target := &fakeForwarderTarget{}
	boom := errors.New("boom")
	forward := func(_ context.Context, _, _ string, _ []byte, _ string) (*webhook.HandleSynchronousWebhookServiceOutput, error) {
		return nil, boom
	}
	wireWebhookForwarder(target, forward)

	if target.fn == nil {
		t.Fatal("expected forwarder to be wired")
	}
	_, _, err := target.fn(context.Background(), "svc", "GET", nil, "")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want %v", err, boom)
	}
}

// TestWireWebhookForwarder_NoopWithoutSetter verifies targets that do not
// implement SetWebhookForwarder are left untouched (no panic, no wiring).
func TestWireWebhookForwarder_NoopWithoutSetter(_ *testing.T) {
	wireWebhookForwarder(struct{}{}, func(_ context.Context, _, _ string, _ []byte, _ string) (*webhook.HandleSynchronousWebhookServiceOutput, error) {
		return nil, nil
	})
}
