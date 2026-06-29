# Component System

Reusable capabilities live under `internal/component` and shared infrastructure lives under `internal/platform`.

## Main Components

| Component | Path | Responsibility |
|-----------|------|----------------|
| AuthSession | `internal/component/authsession` | Bearer auth, JWT session, multi-login, heartbeat offline |
| LoginCrypto | `internal/component/logincrypto` | RSA-OAEP challenge-based password transport |
| IDGen | `internal/component/idgen` | Snowflake IDs, segments, leases, ID codec |
| Redis | `internal/component/eredis` | Redis wrapper and utilities |
| ETUSUpload | `internal/component/etusupload` | TUS resumable upload |

## AuthSession Config

```toml
[app.user]
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true
maxLoginClient = 2
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

## LoginCrypto Config

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
rsaKeyBits = 4096
enableMetrics = true
```

## IDGen Config

```toml
[component.idgen.default]
namespace = "egoadmin-local"
name = "default"

[component.idgen.machine]
group = "egoadmin-local"

[component.idgen.codec]
secret = "local-stable-idcodec-secret"
```

::: warning
`idcodec` is reversible public ID encoding. It is not an authorization mechanism.
:::

## Component Rules

- Components provide reusable infrastructure, not service-specific business rules.
- Keep config minimal.
- Register resource cleanup with `shutdown.Manager`.
- Depend on narrow interfaces in business code.

