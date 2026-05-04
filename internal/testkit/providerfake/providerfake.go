package providerfake

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/review"
)

type Mode string

const (
	ModeStream        Mode = "stream"
	ModeIdle          Mode = "idle"
	ModeEndless       Mode = "endless"
	ModeMutation      Mode = "mutation"
	ModeInvalidPacket Mode = "invalid_packet"
	ModeCrashMid      Mode = "crash_mid_stream"
)

type Provider struct {
	Mode   Mode
	Frames []string
}

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
