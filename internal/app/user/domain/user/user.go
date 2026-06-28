package user

import (
	"strings"
	"time"
)

const (
	// BuiltinRootUsername is the internal super administrator username.
	BuiltinRootUsername = "root"
	// BuiltinAdminUsername is the system administrator username.
	BuiltinAdminUsername = "admin"
	// DefaultPasswordLength is the number of phone suffix digits used for default passwords.
	DefaultPasswordLength = 6
	// TypePlatform is the platform-admin user type.
	TypePlatform int32 = 1
)

// User is the user aggregate root used by new application use cases.
type User struct {
	ID            uint64
	Username      string
	PasswordHash  string
	Name          string
	Phone         string
	Gender        Gender
	Status        Status
	Type          int32
	OnlineStatus  OnlineStatus
	DeptID        uint64
	Remark        string
	RoleIDs       []uint64
	RoleMenus     []string
	HeartbeatTime time.Time
}

type Gender int32

const (
	GenderHidden Gender = iota
	GenderMale
	GenderFemale
)

type Status int32

const (
	StatusUnknown Status = iota
	StatusValid
	StatusInvalid
)

type OnlineStatus int32

const (
	OnlineStatusUnknown OnlineStatus = iota
	OnlineStatusOnline
	OnlineStatusOffline
)

// CanResetPassword reports whether the user can be reset by an administrator.
func (u *User) CanResetPassword() error {
	if u == nil {
		return ErrNotFound
	}
	if IsBuiltinUsername(u.Username) {
		return ErrBuiltinPasswordReset
	}
	return nil
}

// DefaultPassword returns the reset password derived from the user's phone.
func (u *User) DefaultPassword() (string, error) {
	if err := u.CanResetPassword(); err != nil {
		return "", err
	}
	return DefaultPasswordFromPhone(u.Phone)
}

// CanLogin reports whether the user is allowed to authenticate.
func (u *User) CanLogin() error {
	if u == nil {
		return ErrLoginDenied
	}
	if u.Status == StatusInvalid {
		return ErrLoginDenied
	}
	return nil
}

// Menus returns the comma-separated unique menu IDs granted by the user's roles.
func (u *User) Menus() string {
	if u == nil {
		return ""
	}
	seen := make(map[string]struct{}, len(u.RoleMenus))
	menus := make([]string, 0, len(u.RoleMenus))
	for _, raw := range u.RoleMenus {
		for _, menu := range strings.Split(raw, ",") {
			if menu == "" {
				continue
			}
			if _, ok := seen[menu]; ok {
				continue
			}
			seen[menu] = struct{}{}
			menus = append(menus, menu)
		}
	}
	return strings.Join(menus, ",")
}

// DefaultPasswordFromPhone returns the default password derived from a phone number.
func DefaultPasswordFromPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if len(phone) < DefaultPasswordLength {
		return "", ErrInvalidDefaultPasswd
	}
	return phone[len(phone)-DefaultPasswordLength:], nil
}

// IsBuiltinUsername reports whether username belongs to a protected built-in user.
func IsBuiltinUsername(username string) bool {
	return username == BuiltinRootUsername || username == BuiltinAdminUsername
}
