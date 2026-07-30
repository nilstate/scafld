// Package providerpacket shares rejected provider packet material across layers.
package providerpacket

// Source captures provider material scafld could not accept.
type Source struct {
	Provider           string `json:"provider,omitempty"`
	Model              string `json:"model,omitempty"`
	OutputFormat       string `json:"output_format,omitempty"`
	ExpectedSchema     string `json:"expected_schema,omitempty"`
	ExpectedSubmitTool string `json:"expected_submit_tool,omitempty"`
	Error              string `json:"error,omitempty"`
	DiagnosticPath     string `json:"diagnostic_path,omitempty"`
	RejectedText       string `json:"rejected_text,omitempty"`
}
