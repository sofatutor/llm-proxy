package dispatcher

// PermanentBackendError is a custom error type for permanent backend errors (e.g., Helicone 500s that should not be retried)
type PermanentBackendError struct {
	Msg string
}

func (e *PermanentBackendError) Error() string {
	return e.Msg
}
