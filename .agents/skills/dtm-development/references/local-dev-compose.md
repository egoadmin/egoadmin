# Local DTM Compose

## Services

EgoAdmin local development uses:

- `dtm`: shared DTM server.
- `mysql-dtm`: DTM transaction state database.
- `etcd`: EGO service discovery.

Default development runs business services on the host with `make run` or `make run.service`. Run DTM with host networking so it can call host-registered branch services.

DTM image is fixed:

```yaml
image: yedf/dtm:1.19.0
```

Do not use `latest`, `1`, or `1.19`.

## Config File

Prefer mounted `conf.yml`:

```yaml
Store:
  Driver: mysql
  Host: 127.0.0.1
  User: egoadmin
  Password: egoadmin
  Port: 3309
  Db: egoadmin_dtm

MicroService:
  Driver: dtm-driver-ego
  Target: etcd://127.0.0.1:2379/egoadmin-dtm
  EndPoint: 127.0.0.1:36790

HttpPort: 36789
GrpcPort: 36790
```

`test/compose/dtm.yml` should use:

```yaml
network_mode: host
```

Environment variable mapping exists in DTM, for example `MicroService.EndPoint => MICRO_SERVICE_END_POINT`, but use the config file when behavior must be unambiguous.

## Commands

```bash
make dev-up
curl http://127.0.0.1:36789/api/ping
make dev-down
```

For one service:

```bash
make dev.up-one MIDDLEWARE=dtm
make dev.up-one MIDDLEWARE=mysql-dtm
```

## Validation

- Docker Compose config parses.
- `mysql-dtm` becomes healthy.
- `dtm` responds on `/api/ping`.
- DTM registers `egoadmin-dtm` to etcd.
- No DTM containers or data remain after the appropriate cleanup command when a test requires cleanup.
