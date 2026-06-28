package idgen

import "errors"

var (
	ErrInvalidConfig     = errors.New("idgen invalid config")
	ErrStoreUnavailable  = errors.New("idgen segment store unavailable")
	ErrNameNotFound      = errors.New("idgen name not found")
	ErrNameDisabled      = errors.New("idgen name disabled")
	ErrSegmentExhausted  = errors.New("idgen segment exhausted")
	ErrSegmentConflict   = errors.New("idgen segment update conflict")
	ErrOverflow          = errors.New("idgen id overflow")
	ErrComponentClosed   = errors.New("idgen component closed")
	ErrMachineLeaseLost  = errors.New("idgen machine lease lost")
	ErrMachineIDOverflow = errors.New("idgen machine id overflow")
)
