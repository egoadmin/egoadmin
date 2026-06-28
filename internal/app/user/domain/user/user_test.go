package user

import (
	"errors"
	"testing"
)

func TestDefaultPasswordFromPhone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phone    string
		expected string
		wantErr  error
	}{
		{
			name:     "valid phone",
			phone:    "13800138000",
			expected: "138000",
		},
		{
			name:     "trim spaces",
			phone:    " 13800138000 ",
			expected: "138000",
		},
		{
			name:    "short phone",
			phone:   "12345",
			wantErr: ErrInvalidDefaultPasswd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DefaultPasswordFromPhone(tt.phone)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DefaultPasswordFromPhone() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.expected {
				t.Fatalf("DefaultPasswordFromPhone() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUserDefaultPasswordRejectsBuiltinUser(t *testing.T) {
	t.Parallel()

	_, err := (&User{
		Username: BuiltinAdminUsername,
		Phone:    "13800138000",
	}).DefaultPassword()
	if !errors.Is(err, ErrBuiltinPasswordReset) {
		t.Fatalf("DefaultPassword() error = %v, want ErrBuiltinPasswordReset", err)
	}
}

func TestUserMenus(t *testing.T) {
	t.Parallel()

	u := &User{
		RoleMenus: []string{
			"dashboard,user",
			"user,role,,dept",
		},
	}
	if got := u.Menus(); got != "dashboard,user,role,dept" {
		t.Fatalf("Menus() = %q, want dashboard,user,role,dept", got)
	}
}
