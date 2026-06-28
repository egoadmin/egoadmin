# eredis

This package is the egoadmin Redis component. It is based on `github.com/redis/go-redis/v9` and replaces direct use of `github.com/gotomicro/ego-component/eredis` in egoadmin code.

Application code should normally depend on `internal/platform/cache/redis` or a service-owned cache package such as `internal/app/user/internal/cache`, not this package directly. The wrapper keeps token, lock, and health-check behavior stable while the underlying Redis component evolves.

## Config

```toml
[client.redis]
addr = "127.0.0.1:6379"
mode = "stub"
password = "change-me"
debug = true
```

Supported modes:

- `stub`
- `cluster`
- `sentinel`
- `ring`

## Usage

```go
rd := eredis.Load("client.redis").Build()
value, err := rd.Get(ctx, "key")
```

The component includes debug/access/metric/trace hooks and a Redis-backed ecron lock implementation under `ecronlock`.
