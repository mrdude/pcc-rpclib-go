package apirt

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mrdude/pcc-rpclib-go/apivalidation"
)

type Exception struct {
	err error

	status  int
	message string
}

func NewException(status int, message string) error {
	return &Exception{status: status, message: message}
}

func NewWrappedException(status int, message string, err error) error {
	return &Exception{status: status, message: message, err: err}
}

func CoerceErrorToException(err error) *Exception {
	if err == nil {
		return nil
	}

	// exception
	var ex *Exception
	if errors.As(err, &ex) {
		return ex
	}

	// common errors
	if errors.Is(err, context.DeadlineExceeded) {
		return &Exception{
			err:     err,
			status:  http.StatusTooManyRequests,
			message: "request timed out",
		}
	} else if errors.Is(err, &apivalidation.Error{}) {
		return &Exception{
			err:     err,
			status:  http.StatusBadRequest,
			message: err.Error(),
		}
	}

	return &Exception{
		err:    err,
		status: http.StatusInternalServerError,
	}
}

func (ex *Exception) Error() string {
	if ex.err != nil {
		return fmt.Sprintf("[%d] %s: %s", ex.EffectiveStatus(), ex.EffectiveMessage(), ex.err)
	}

	return fmt.Sprintf("[%d] %s", ex.EffectiveStatus(), ex.EffectiveMessage())
}

func (ex *Exception) Unwrap() error {
	return ex.err
}

func (ex *Exception) EffectiveMessage() string {
	if ex.message == "" {
		return http.StatusText(ex.EffectiveStatus())
	}

	return ex.message
}

func (ex *Exception) EffectiveStatus() int {
	if ex.status == 0 {
		return http.StatusInternalServerError
	}

	return ex.status
}
