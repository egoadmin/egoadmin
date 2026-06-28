package service

import userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"

// DefaultPasswordFromPhone returns the default password derived from a phone number.
func DefaultPasswordFromPhone(phone string) (string, error) {
	return userdomain.DefaultPasswordFromPhone(phone)
}
