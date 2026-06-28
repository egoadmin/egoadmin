package store

import "time"

// ZeroTime is the sentinel value used by datetime fields to represent "unset".
var ZeroTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)

// IsZeroTime reports whether t is unset, accepting both Go zero time and ZeroTime.
func IsZeroTime(t time.Time) bool {
	return t.IsZero() || t.Equal(ZeroTime)
}
