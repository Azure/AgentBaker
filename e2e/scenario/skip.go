package scenario

type skipError struct {
	message string
}

func (e *skipError) Error() string {
	return e.message
}
