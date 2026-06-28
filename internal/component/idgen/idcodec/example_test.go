package idcodec_test

import (
	"fmt"

	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
)

func ExampleComponent() {
	cfg := idcodec.DefaultConfig()
	cfg.Secret = "0123456789abcdef0123456789abcdef"
	cfg.EnableMetrics = false

	codec := idcodec.DefaultContainer().Build(idcodec.WithConfig(cfg))
	publicID, _ := codec.Encode("order", 2)
	id, _ := codec.DecodeWithPrefix("order", publicID)

	fmt.Println(publicID)
	fmt.Println(id)
	// Output:
	// order-07uQlcBmL6d0
	// 2
}
