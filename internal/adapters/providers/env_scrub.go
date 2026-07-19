package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProviderEnvInput describes the host environment and committed endpoint pins
// allowed to reach a receipt-grade reviewer subprocess.
type ProviderEnvInput struct {
	Provider     string
	HostEnviron  []string
	ExtraEnv     []string
	EndpointURL  string
	EndpointHost string
	RequireAuth  bool
	// MemoryHome, when set, overrides HOME and XDG_CONFIG_HOME with a clean
	// sandbox-owned directory so the reviewer cannot autoload host user-global
	// agent memory.
	MemoryHome string
}

// ProviderEnvResult is the exact process env plus the endpoint host stamped
// into receipt runtime facts.
type ProviderEnvResult struct {
	Env          []string
	EndpointHost string
}

// ReceiptGradeBinary is the absolute reviewer binary path and byte digest.
type ReceiptGradeBinary struct {
	Path   string
	SHA256 string
}

// ScrubProviderEnv returns the exact env for a receipt-grade reviewer process.
func ScrubProviderEnv(input ProviderEnvInput) (ProviderEnvResult, error) {
	provider := normalizeReviewerProvider(input.Provider)
	if provider == "" {
		return ProviderEnvResult{}, fmt.Errorf("unknown receipt-grade provider %q", input.Provider)
	}
	endpointHost, err := effectiveEndpointHost(provider, input.EndpointURL, input.EndpointHost)
	if err != nil {
		return ProviderEnvResult{}, err
	}
	values := map[string]string{}
	for _, entry := range append(append([]string(nil), input.HostEnviron...), input.ExtraEnv...) {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		upper := strings.ToUpper(key)
		switch {
		case isProxyEnvKey(upper):
			continue
		case isEndpointEnvKey(upper):
			if endpointValueMatchesHost(value, endpointHost) {
				values[key] = value
			}
			continue
		case isCommonAllowedEnvKey(upper) || isProviderAuthEnvKey(provider, upper):
			values[key] = value
		}
	}
	if home := strings.TrimSpace(input.MemoryHome); home != "" {
		values["HOME"] = home
		values["XDG_CONFIG_HOME"] = filepath.Join(home, ".config")
	}
	if endpointURL := strings.TrimSpace(input.EndpointURL); endpointURL != "" {
		values[providerEndpointEnvKey(provider)] = endpointURL
	}
	if input.RequireAuth && !hasProviderAuth(provider, values) {
		return ProviderEnvResult{}, fmt.Errorf("missing required auth env for %s", provider)
	}
	return ProviderEnvResult{Env: sortedEnv(values), EndpointHost: endpointHost}, nil
}

func providerEndpointEnvKey(provider string) string {
	switch provider {
	case HostAgentClaude:
		return "ANTHROPIC_BASE_URL"
	case HostAgentGemini:
		return "GOOGLE_GEMINI_BASE_URL"
	default:
		return "OPENAI_BASE_URL"
	}
}

// ResolveReceiptGradeBinary fails closed unless path is absolute, executable,
// and hashable without PATH lookup.
func ResolveReceiptGradeBinary(path string) (ReceiptGradeBinary, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ReceiptGradeBinary{}, fmt.Errorf("receipt-grade reviewer binary is required")
	}
	if !filepath.IsAbs(path) {
		return ReceiptGradeBinary{}, fmt.Errorf("receipt-grade reviewer binary must be an absolute path: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ReceiptGradeBinary{}, fmt.Errorf("stat receipt-grade reviewer binary: %w", err)
	}
	if info.IsDir() {
		return ReceiptGradeBinary{}, fmt.Errorf("receipt-grade reviewer binary is a directory: %s", path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return ReceiptGradeBinary{}, fmt.Errorf("receipt-grade reviewer binary is not executable: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ReceiptGradeBinary{}, fmt.Errorf("hash receipt-grade reviewer binary: %w", err)
	}
	sum := sha256.Sum256(data)
	return ReceiptGradeBinary{Path: path, SHA256: hex.EncodeToString(sum[:])}, nil
}

func effectiveEndpointHost(provider, endpointURL, endpointHost string) (string, error) {
	endpointURL = strings.TrimSpace(endpointURL)
	endpointHost = strings.TrimSpace(endpointHost)
	if endpointURL != "" && endpointHost != "" {
		return "", fmt.Errorf("set endpoint_url or endpoint_host for %s, not both", provider)
	}
	if endpointURL != "" {
		host, err := hostFromEndpointValue(endpointURL)
		if err != nil {
			return "", err
		}
		return host, nil
	}
	if endpointHost != "" {
		return normalizeEndpointHost(endpointHost), nil
	}
	return defaultEndpointHost(provider), nil
}

func defaultEndpointHost(provider string) string {
	switch provider {
	case "claude":
		return "api.anthropic.com"
	case "gemini":
		return "generativelanguage.googleapis.com"
	default:
		return "api.openai.com"
	}
}

func endpointValueMatchesHost(value, endpointHost string) bool {
	host, err := hostFromEndpointValue(value)
	return err == nil && host == normalizeEndpointHost(endpointHost)
}

func hostFromEndpointValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("endpoint value is empty")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse endpoint value %q: %w", value, err)
	}
	host := parsed.Host
	if host == "" && parsed.Scheme == "" {
		host = parsed.Path
	}
	host = normalizeEndpointHost(host)
	if host == "" {
		return "", fmt.Errorf("endpoint value has no host: %q", value)
	}
	return host, nil
}

func normalizeEndpointHost(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func isProxyEnvKey(upper string) bool {
	switch upper {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
		return true
	default:
		return false
	}
}

func isEndpointEnvKey(upper string) bool {
	return strings.HasSuffix(upper, "_BASE_URL") ||
		strings.HasSuffix(upper, "_API_BASE") ||
		strings.HasSuffix(upper, "_ENDPOINT")
}

func isCommonAllowedEnvKey(upper string) bool {
	switch upper {
	case "HOME", "PATH", "TMPDIR", "TMP", "TEMP", "XDG_CONFIG_HOME":
		return true
	default:
		return false
	}
}

func isProviderAuthEnvKey(provider, upper string) bool {
	switch provider {
	case "claude":
		return upper == "ANTHROPIC_API_KEY"
	case "gemini":
		return upper == "GEMINI_API_KEY" || upper == "GOOGLE_API_KEY"
	default:
		return upper == "OPENAI_API_KEY" || upper == "CODEX_HOME"
	}
}

func hasProviderAuth(provider string, env map[string]string) bool {
	provider = normalizeReviewerProvider(provider)
	if provider == HostAgentCodex {
		if strings.TrimSpace(env["OPENAI_API_KEY"]) != "" {
			return true
		}
		home := strings.TrimSpace(env["CODEX_HOME"])
		info, err := os.Stat(filepath.Join(home, "auth.json"))
		return home != "" && err == nil && !info.IsDir() && info.Size() > 0
	}
	for key, value := range env {
		if strings.TrimSpace(value) != "" && isProviderAuthEnvKey(provider, strings.ToUpper(key)) {
			return true
		}
	}
	return false
}

func sortedEnv(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}
