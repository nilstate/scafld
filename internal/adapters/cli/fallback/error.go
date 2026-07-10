package fallback

// Error returns err when present, otherwise fallback.
func Error(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
