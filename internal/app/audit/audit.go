package audit

func Scope(paths []string) []string {
	return append([]string(nil), paths...)
}
