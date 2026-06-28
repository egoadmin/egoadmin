# EgoAdmin Deploy Compose

This directory is a single-machine deployment example for the current EgoAdmin microservice topology.

## Layout

- `docker-compose.yml` is the entrypoint.
- `compose/app.yml` runs `idgen`, `user`, and `gateway`.
- `compose/mysql.yml` and `compose/redis.yml` keep service-owned data stores separate.
- `compose/dtm.yml` runs one shared DTM server with its own `mysql-dtm`.
- `configs/<service>/config.toml` contains container-network runtime config.
- `data/` is local persistent data and is ignored by Git.
- `data/dtm/conf.yml` is generated from `deploy/.env` by `make deploy.prepare`.

Service config files are intentionally small. Prefer `deploy/.env` and `EGOADMIN_*` overrides for environment-specific addresses and secrets, and add TOML entries only when the default config is not valid for container deployment.

## Usage

```bash
cp deploy/.env.example deploy/.env
make image.build DOCKER_TAG=latest
make deploy-config
make deploy-up
make deploy-ps
make deploy-down
```

`deploy` mode assumes business services and middleware run in the same Compose network. DTM therefore uses `dtm:36790` and `etcd://etcd:2379/egoadmin-dtm`. This differs from `test/compose/dtm.yml`, where local development normally runs business services on the host and DTM uses host networking.

Service discovery relies on the built-in defaults for `registry.scheme`, `registry.prefix`, and `registry.serviceTTL`. In deploy mode we only override `etcd.addrs` to point at the shared `etcd` node, and `server.grpc.enableLocalMainIP` so registered gRPC addresses are reachable from other containers.

Override credentials and public ports in `deploy/.env`; keep real production secrets outside version control.
