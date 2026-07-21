package corebundle

import "fmt"

// CorePrompt returns an embedded managed core prompt by filename, such as
// "review.md".
func CorePrompt(filename string) ([]byte, error) {
	data, err := assets.ReadFile("assets/core/prompts/" + filename)
	if err != nil {
		return nil, fmt.Errorf("read embedded core prompt %s: %w", filename, err)
	}
	return data, nil
}
