package api

import "strings"

// API represents a gateway-facing gRPC API catalog entry.
type API struct {
	ID       uint64
	Signcode string
	Name     string
	Path     string
	Method   string
}

// FullPath returns the permission identity used by frontend contracts and Casbin.
func (a API) FullPath() string {
	return strings.ToUpper(strings.TrimSpace(a.Path) + "/" + strings.TrimSpace(a.Method))
}

// Changed reports whether persisted mutable fields differ from another API entry.
func (a API) Changed(next API) bool {
	return a.Name != next.Name ||
		a.Path != next.Path ||
		a.Method != next.Method
}
