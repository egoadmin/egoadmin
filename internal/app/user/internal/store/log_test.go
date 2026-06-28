package store

import "testing"

func TestLogModelIDConversionsPreferNumericColumns(t *testing.T) {
	t.Parallel()

	model := &LogModel{
		UserID:    "12",
		UserIDU64: 34,
		DeptID:    "56",
		DeptIDU64: 78,
	}
	if got := model.UserIdToRPC(); got != 34 {
		t.Fatalf("UserIdToRPC() = %d, want 34", got)
	}
	if got := model.DeptIdToRPC(); got != 78 {
		t.Fatalf("DeptIdToRPC() = %d, want 78", got)
	}
}

func TestLogModelIDConversionsFallbackToString(t *testing.T) {
	t.Parallel()

	model := &LogModel{
		UserID: "12",
		DeptID: "56",
	}
	if got := model.UserIdToRPC(); got != 12 {
		t.Fatalf("UserIdToRPC() = %d, want 12", got)
	}
	if got := model.DeptIdToRPC(); got != 56 {
		t.Fatalf("DeptIdToRPC() = %d, want 56", got)
	}
}
