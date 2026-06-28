package service

import "testing"

func TestDefaultPasswordFromPhone(t *testing.T) {
	t.Parallel()

	got, err := DefaultPasswordFromPhone("13800138000")
	if err != nil {
		t.Fatalf("DefaultPasswordFromPhone() error = %v", err)
	}
	if got != "138000" {
		t.Fatalf("DefaultPasswordFromPhone() = %q, want %q", got, "138000")
	}
}

func TestDefaultPasswordFromPhoneRejectsShortPhone(t *testing.T) {
	t.Parallel()

	_, err := DefaultPasswordFromPhone("12345")
	if err == nil {
		t.Fatal("DefaultPasswordFromPhone() expected error")
	}
}
