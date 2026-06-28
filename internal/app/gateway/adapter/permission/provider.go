package permission

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewPolicyCleaner,
)
