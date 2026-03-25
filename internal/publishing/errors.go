package publishing

import "errors"

var (
	ErrInvalidScheduleInput = errors.New("invalid publishing schedule input")
	ErrScheduleNotFound     = errors.New("publishing schedule not found")
)
