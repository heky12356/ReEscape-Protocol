package agent

import "errors"

var (
	ErrEmptyModelResponse = errors.New("agent model response is empty")
	ErrMaxStepsExceeded   = errors.New("agent max steps exceeded")
)
