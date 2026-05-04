package audit

// Scope returns a copy of paths selected for scope auditing.
func Scope(paths []string) []string {
	return append([]string(nil), paths...)
}
