package providerfake

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/review"
)

func TestProviderFakeModes(t *testing.T) {
	t.Parallel()

	t.Run("stream", func(t *testing.T) {
		var out bytes.Buffer
		err := Provider{Mode: ModeStream, Frames: []string{`{"type":"done"}`}}.Run(context.Background(), &out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "done") {
			t.Fatalf("output %q does not contain frame", out.String())
		}
	})

	t.Run("crash mid stream", func(t *testing.T) {
		var out bytes.Buffer
		err := Provider{Mode: ModeCrashMid}.Run(context.Background(), &out)
		if err == nil {
			t.Fatal("expected crash error")
		}
		if !strings.Contains(out.String(), "partial") {
			t.Fatalf("output %q does not contain partial frame", out.String())
		}
	})

	t.Run("idle respects context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		err := Provider{Mode: ModeIdle}.Run(ctx, &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected context error")
		}
	})
}

func TestProviderFakeIdleTimeoutEndlessStreamInvalidPacketCrashMidStreamMutation(t *testing.T) {
	t.Parallel()
	for _, mode := range []Mode{ModeMutation, ModeInvalidPacket, ModeCrashMid} {
		var out bytes.Buffer
		_ = Provider{Mode: mode}.Run(context.Background(), &out)
		if out.Len() == 0 {
			t.Fatalf("%s produced no output", mode)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_ = Provider{Mode: ModeEndless}.Run(ctx, &bytes.Buffer{})
}

func TestProviderFakeInvokeParsesPacketsAndSurfacesInvalidOutput(t *testing.T) {
	t.Parallel()

	packet, err := Provider{Mode: ModeMutation}.Invoke(context.Background(), review.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Verdict != "fail" {
		t.Fatalf("mutation packet verdict = %q", packet.Verdict)
	}

	_, err = Provider{Mode: ModeInvalidPacket}.Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, review.ErrInvalidPacket) {
		t.Fatalf("invalid packet err = %v", err)
	}
}
