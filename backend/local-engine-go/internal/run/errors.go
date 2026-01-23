package run

type ValidationError struct {
	Message string
	Details string
}

func (e ValidationError) Error() string {
	if e.Details == "" {
		return e.Message
	}
	return e.Message + ": " + e.Details
}

type NotFoundError struct {
	Message string
	Details string
}

func (e NotFoundError) Error() string {
	if e.Details == "" {
		return e.Message
	}
	return e.Message + ": " + e.Details
}

type ConflictError struct {
	Message string
	Details string
}

func (e ConflictError) Error() string {
	if e.Details == "" {
		return e.Message
	}
	return e.Message + ": " + e.Details
}
