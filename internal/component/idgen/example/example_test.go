// Package example contains executable idgen component examples.
package example

import (
	"context"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	idgenredis "github.com/egoadmin/egoadmin/internal/component/idgen/machine/redis"
	"github.com/egoadmin/egoadmin/internal/component/idgen/store/gormstore"
	"github.com/gotomicro/ego/core/elog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	demoNamespace = "demo"
	defaultName   = "default"
)

func Example_defaultInstance() {
	store := idgen.NewMemoryStore()
	manager := exampleMachineManager{}
	cfg := exampleConfig(defaultName)

	gen := idgen.DefaultContainer().Build(
		idgen.WithName("component.idgen.default"),
		idgen.WithConfig(cfg),
		idgen.WithSegmentStore(store),
		idgen.WithMachineLeaseManager(manager),
	)
	if err := gen.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if stopErr := gen.Stop(); stopErr != nil {
			panic(stopErr)
		}
	}()

	id, err := gen.NextDefault(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
	// Output: 1
}

func Example_multiInstanceSharedManager() {
	store := idgen.NewMemoryStore()
	manager := exampleMachineManager{}
	defaultCfg := exampleConfig(defaultName)
	orderCfg := exampleConfig("order")

	defaultGen := idgen.DefaultContainer().Build(
		idgen.WithName("component.idgen.default"),
		idgen.WithConfig(defaultCfg),
		idgen.WithSegmentStore(store),
		idgen.WithMachineLeaseManager(manager),
	)
	orderGen := idgen.DefaultContainer().Build(
		idgen.WithName("component.idgen.order"),
		idgen.WithConfig(orderCfg),
		idgen.WithSegmentStore(store),
		idgen.WithMachineLeaseManager(manager),
	)
	for _, gen := range []*idgen.Component{defaultGen, orderGen} {
		if err := gen.Start(); err != nil {
			panic(err)
		}
		defer func(gen *idgen.Component) {
			if stopErr := gen.Stop(); stopErr != nil {
				panic(stopErr)
			}
		}(gen)
	}

	defaultID, err := defaultGen.NextDefault(context.Background())
	if err != nil {
		panic(err)
	}
	orderID, err := orderGen.NextDefault(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(defaultID, orderID)
	// Output: 1 1
}

func businessRedisInjectionExample(db *gorm.DB, redisComponent *eredis.Component) {
	allocator := idgenredis.New(redisComponent.Client())
	machineCfg := idgen.DefaultMachineConfig()
	machineCfg.Group = "egoadmin"

	manager, err := idgen.NewMachineLeaseManager(
		"component.idgen.machine",
		machineCfg,
		allocator,
		elog.EgoLogger.With(elog.FieldComponent(idgen.PackageName), elog.FieldComponentName("component.idgen.machine")),
	)
	if err != nil {
		panic(err)
	}
	if err = manager.Start(context.Background()); err != nil {
		panic(err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if stopErr := manager.Stop(stopCtx); stopErr != nil {
			panic(stopErr)
		}
	}()

	store := gormstore.New(db)
	gen := idgen.Load("component.idgen.default").Build(
		idgen.WithSegmentStore(store),
		idgen.WithMachineLeaseManager(manager),
	)
	if err = gen.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if stopErr := gen.Stop(); stopErr != nil {
			panic(stopErr)
		}
	}()
}

func Example_gormStoreAutoEnsure() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err = db.AutoMigrate(&gormstore.SegmentModel{}); err != nil {
		panic(err)
	}
	cfg := exampleConfig(defaultName)

	gen := idgen.DefaultContainer().Build(
		idgen.WithName("component.idgen.default"),
		idgen.WithConfig(cfg),
		idgen.WithSegmentStore(gormstore.New(db)),
		idgen.WithMachineLeaseManager(exampleMachineManager{}),
	)
	if err = gen.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if stopErr := gen.Stop(); stopErr != nil {
			panic(stopErr)
		}
	}()

	id, err := gen.NextDefault(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
	// Output: 1
}

func Example_metricsLowCardinality() {
	cfg := exampleConfig(defaultName)
	cfg.EnableMetrics = true
	cfg.EnableNameMetricLabel = false

	store := idgen.NewMemoryStore()
	gen := idgen.DefaultContainer().Build(
		idgen.WithName("component.idgen.default"),
		idgen.WithConfig(cfg),
		idgen.WithSegmentStore(store),
		idgen.WithMachineLeaseManager(exampleMachineManager{}),
	)
	if err := gen.Start(); err != nil {
		panic(err)
	}
	defer func() {
		if stopErr := gen.Stop(); stopErr != nil {
			panic(stopErr)
		}
	}()

	fmt.Println(cfg.EnableMetrics, cfg.EnableNameMetricLabel)
	// Output: true false
}

func exampleConfig(name string) *idgen.Config {
	cfg := idgen.DefaultConfig()
	cfg.Namespace = demoNamespace
	cfg.Name = name
	cfg.Step = 1000
	cfg.MinStep = 1000
	cfg.MaxStep = 100000
	cfg.EnableMetrics = false
	return cfg
}

type exampleMachineManager struct{}

func (exampleMachineManager) Start(context.Context) error {
	return nil
}

func (exampleMachineManager) Stop(context.Context) error {
	return nil
}

func (exampleMachineManager) Renew(context.Context) error {
	return nil
}

func (exampleMachineManager) Lease() (idgen.MachineLease, bool) {
	return idgen.MachineLease{
		Namespace:  demoNamespace,
		InstanceID: "example-instance",
		SessionID:  "example-session",
		MachineID:  0,
		TTL:        time.Minute,
		ExpiresAt:  time.Now().Add(time.Minute),
	}, true
}

func (exampleMachineManager) Health(context.Context) error {
	return nil
}

func (exampleMachineManager) LostPolicy() idgen.LostPolicy {
	return idgen.LostPolicyFailClosed
}

func (exampleMachineManager) LeaseLost() bool {
	return false
}
