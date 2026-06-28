package idcodec

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewIDCodecComponent,
)

func NewIDCodecComponent() *Component {
	return Load(defaultComponentName).Build()
}
