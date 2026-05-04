package providerfake

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/review"
)

// Mode selects the fake provider behavior.
type Mode string

const (
	// ModeStream emits configured frames and exits successfully.
	ModeStream Mode = "stream"
	// ModeIdle blocks until context cancellation.
	ModeIdle Mode = "idle"
	// ModeEndless streams tick events until context cancellation.
	ModeEndless Mode = "endless"
	// ModeMutation emits a workspace mutation event.
	ModeMutation Mode = "mutation"
	// ModeInvalidPacket emits malformed provider output.
	ModeInvalidPacket Mode = "invalid_packet"
	// ModeCrashMid emits a partial frame and then fails.
	ModeCrashMid Mode = "crash_mid_stream"
)

// Provider is a deterministic review provider fake.
type Provider struct {
	Mode   Mode
	Frames []string
}

// Run writes fake provider output to w according to Mode.
func (p Provider) Run(ctx context.Context, w io.Writer) error {
	switch p.Mode {
	case ModeStream:
		for _, frame := range p.Frames {
			if _, err := io.WriteString(w, frame+"\n"); err != nil {
				return err
			}
		}
		return nil
	case ModeIdle:
		<-ctx.Done()
		return ctx.Err()
	case ModeEndless:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if _, err := io.WriteString(w, `{"type":"tick"}`+"\n"); err != nil {
					return err
				}
				time.Sleep(time.Millisecond)
			}
		}
	case ModeMutation:
		_, err := io.WriteString(w, `{"type":"workspace_mutation"}`+"\n")
		return err
	case ModeInvalidPacket:
		_, err := io.WriteString(w, "{invalid\n")
		return err
	case ModeCrashMid:
		_, _ = io.WriteString(w, `{"type":"partial"}`+"\n")
		return errors.New("provider crashed mid-stream")
	default:
		return errors.New("unknown provider fake mode")
	}
}

// Invoke runs the fake provider and parses its packet output.
func (p Provider) Invoke(ctx context.Context, req review.Request) (review.Packet, error) {
	var out bytes.Buffer
	err := p.Run(ctx, &out)
	if out.Len() == 0 {
		return review.Packet{}, err
	}
	packet, parseErr := review.ParseNDJSON(out.String())
	if parseErr != nil {
		return review.Packet{}, parseErr
	}
	if err != nil {
		return review.Packet{}, err
	}
	return packet, nil
}
