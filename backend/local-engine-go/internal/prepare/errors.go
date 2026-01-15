package prepare

import "fmt"

type ValidationError struct {
	Code    string
	Message string
	Details string
}

func (e ValidationError) Error() string {
	if e.Details == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Message, e.Details)
}

func (e ValidationError) Response() *ErrorResponse {
	return errorResponse(e.Code, e.Message, e.Details)
}

func ToErrorResponse(err error) *ErrorResponse {
	if err == nil {
		return nil
	}
	switch v := err.(type) {
	case ValidationError:
		return v.Response()
	case *ValidationError:
		return v.Response()
	default:
		return errorResponse("internal_error", "internal error", err.Error())
	}
}

func errorResponse(code, message, details string) *ErrorResponse {
	resp := &ErrorResponse{
		Code:    code,
		Message: message,
	}
	if details != "" {
		resp.Details = details
	}
	return resp
}
