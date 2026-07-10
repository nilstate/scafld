package review

import (
	"context"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
)

type providerReviewInput struct {
	taskID string
	prompt string
	packet reviewcontext.Packet
}

func invokeReviewProvider(ctx context.Context, provider Provider, in providerReviewInput) (corereview.Dossier, error) {
	return provider.Invoke(ctx, corereview.Request{TaskID: in.taskID, Prompt: in.prompt, Context: in.packet})
}
