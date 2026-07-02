# Поток CLI Auth

Этот документ описывает interaction flow для CLI-среза Google OIDC auth.

Он следует утвержденному CLI-синтаксису в
[`../user-guides/sqlrs-auth.md`](../user-guides/sqlrs-auth.md) и принятому
решению в
[`../adr/2026-07-01-google-oidc-cli-auth.md`](../adr/2026-07-01-google-oidc-cli-auth.md).

Этот срез не вводит изменений sqlrs HTTP API. Gateway по-прежнему получает
только short-lived Google ID token как bearer token.

## 1. Scope

В scope:

- `sqlrs auth login google`
- `sqlrs auth status`
- `sqlrs auth logout`
- effective bearer-token resolution для protected remote API commands

Вне scope:

- server-side refresh-token storage;
- изменения local engine auth;
- новые user/org API endpoint-ы;
- OIDC providers кроме Google;
- device code flow, если loopback login позже не окажется impractical.

## 2. Участники

- **User** - вызывает `sqlrs auth` или protected remote command.
- **CLI parser** - разбирает global flags, profile, output mode и auth
  subcommand arguments.
- **Profile resolver** - загружает выбранный profile, endpoint, `auth.mode`,
  client ID, issuer и имя debug override environment variable.
- **Auth resolver** - владеет auth-session decisions для одного CLI invocation:
  приоритет `SQLRS_TOKEN`, проверки expiry cached ID token, refresh и
  login-required errors.
- **Loopback listener** - слушает `127.0.0.1:<random-port>` во время login и
  получает Google authorization callback.
- **Browser** - открывает Google authorization URL для user consent.
- **Google Authorization Endpoint** - возвращает authorization code через
  loopback redirect.
- **Google Token Endpoint** - обменивает authorization code и refresh token на
  ID token-ы.
- **Google Revocation Endpoint** - revoke-ит refresh token во время logout,
  если это возможно.
- **OS Credential Store** - хранит refresh token и optional cached ID token:
  Windows Credential Manager, macOS Keychain или Linux Secret Service/libsecret.
- **HTTP client** - отправляет sqlrs API requests с effective bearer token.
- **Gateway** - проверяет short-lived Google ID token и выводит actor claims.
- **Renderer** - печатает human или JSON output без raw token-ов.

## 3. Flow: `sqlrs auth login google`

```mermaid
sequenceDiagram
  autonumber
  participant USER as User
  participant CLI as CLI
  participant PROFILE as Profile resolver
  participant LPB as Loopback listener
  participant BROWSER as Browser
  participant GOOGLE_AUTH as Google Authorization Endpoint
  participant GOOGLE_TOKEN as Google Token Endpoint
  participant STORE as OS Credential Store
  participant RENDER as Renderer

  USER->>CLI: sqlrs auth login google
  CLI->>PROFILE: resolve selected profile
  PROFILE-->>CLI: remote endpoint + auth.mode=oidcSession + clientID + issuer
  CLI->>CLI: generate PKCE verifier/challenge, state, nonce
  CLI->>LPB: listen on loopback random port
  CLI->>BROWSER: open Google authorization URL
  BROWSER->>GOOGLE_AUTH: user consent
  GOOGLE_AUTH-->>LPB: redirect with code + state
  LPB-->>CLI: callback query
  CLI->>CLI: validate state and callback parameters
  CLI->>GOOGLE_TOKEN: exchange code + PKCE verifier
  GOOGLE_TOKEN-->>CLI: id_token + refresh_token + expiry
  CLI->>CLI: validate id_token iss, aud, exp, nonce
  CLI->>STORE: save refresh token and session metadata
  CLI->>RENDER: safe login summary
  RENDER-->>USER: logged in
```

Rules:

- Callback принимается только на `127.0.0.1`.
- `state` mismatch, OAuth `error` или missing `code` завершают login до token
  exchange.
- Missing `refresh_token` завершает login с troubleshooting hint. CLI просит у
  Google offline access через `access_type=offline` и `prompt=consent`.
- Refresh token хранится только в OS credential store.
- Raw refresh token-ы и raw ID token-ы никогда не печатаются.

## 4. Flow: Protected Remote API Token Resolution

```mermaid
sequenceDiagram
  autonumber
  participant USER as User
  participant CLI as CLI
  participant PROFILE as Profile resolver
  participant AUTH as Auth resolver
  participant STORE as OS Credential Store
  participant GOOGLE_TOKEN as Google Token Endpoint
  participant CLIENT as HTTP client
  participant GW as Gateway

  USER->>CLI: sqlrs protected remote command
  CLI->>PROFILE: resolve selected profile
  PROFILE-->>CLI: remote endpoint + auth settings
  CLI->>AUTH: resolve effective bearer token
  alt SQLRS_TOKEN is set
    AUTH-->>CLI: token from environment override
  else auth.mode is oidcSession
    AUTH->>STORE: load local session
    STORE-->>AUTH: refresh token + cached ID token metadata
    alt cached ID token is fresh
      AUTH-->>CLI: cached ID token
    else cached ID token missing or expiring soon
      AUTH->>GOOGLE_TOKEN: refresh_token grant
      GOOGLE_TOKEN-->>AUTH: new ID token + expiry
      AUTH->>STORE: update cached ID token metadata
      AUTH-->>CLI: new ID token
    end
  else legacy bearer profile
    AUTH-->>CLI: static profile token or no-token error
  end
  CLI->>CLIENT: protected API request + bearer token
  CLIENT->>GW: Authorization bearer ID token
  GW-->>CLIENT: API response
  CLIENT-->>CLI: result or mapped error
  CLI-->>USER: rendered command output
```

Rules:

- `SQLRS_TOKEN` имеет приоритет над stored sessions и static profile token-ами.
- OIDC sessions refresh-ят cached ID token, когда он missing, expired или
  истекает в течение пяти минут.
- Refresh-token failures останавливают команду до protected sqlrs API request
  и предлагают пользователю выполнить `sqlrs auth login google`.
- Gateway получает только effective bearer token. Он никогда не получает
  refresh token.

## 5. Flow: `sqlrs auth status`

```mermaid
sequenceDiagram
  autonumber
  participant USER as User
  participant CLI as CLI
  participant PROFILE as Profile resolver
  participant AUTH as Auth resolver
  participant STORE as OS Credential Store
  participant RENDER as Renderer

  USER->>CLI: sqlrs auth status
  CLI->>PROFILE: resolve selected profile
  PROFILE-->>CLI: profile name + endpoint + auth settings
  CLI->>AUTH: inspect override and stored session
  AUTH->>STORE: read session metadata when needed
  STORE-->>AUTH: safe metadata or not found
  AUTH-->>CLI: logged-in status + safe claim summary
  CLI->>RENDER: human or JSON status
  RENDER-->>USER: status without raw tokens
```

Rules:

- Status показывает `logged in` или `not logged in`, provider, email, issuer,
  audience/client ID, token expiry, profile, endpoint и override source.
- Если `SQLRS_TOKEN` задан, status показывает override без вывода его value.
- Verbose output может включать только safe claim summary fields: `iss`, `aud`,
  masked `sub`, `email` и `exp`.

## 6. Flow: `sqlrs auth logout`

```mermaid
sequenceDiagram
  autonumber
  participant USER as User
  participant CLI as CLI
  participant PROFILE as Profile resolver
  participant STORE as OS Credential Store
  participant GOOGLE_REVOKE as Google Revocation Endpoint
  participant RENDER as Renderer

  USER->>CLI: sqlrs auth logout
  CLI->>PROFILE: resolve selected profile
  PROFILE-->>CLI: credential lookup scope
  CLI->>STORE: load refresh token if present
  STORE-->>CLI: refresh token or not found
  opt refresh token present and revoke enabled
    CLI->>GOOGLE_REVOKE: revoke refresh token
    GOOGLE_REVOKE-->>CLI: revocation result
  end
  CLI->>STORE: delete local credential
  STORE-->>CLI: delete result
  CLI->>RENDER: safe logout summary
  RENDER-->>USER: logged out
```

Rules:

- `logout` удаляет local credentials, даже если Google revocation failed.
- `--no-revoke` пропускает Google revocation request.
- `logout` не unset-ит и не меняет `SQLRS_TOKEN`.
- Команда idempotent, когда local session отсутствует.

## 7. Failure Handling

| Failure | Behavior |
| --- | --- |
| Local profile selected | Fail before opening browser or reading credentials. |
| `auth.mode` is not `oidcSession` for login | Fail with profile configuration guidance. |
| Credential store unavailable | Fail without plaintext refresh-token fallback. |
| Callback `state` mismatch | Fail login and discard callback data. |
| Callback contains OAuth `error` | Fail login with the provider error summary. |
| Callback is missing `code` | Fail login before token exchange. |
| Token endpoint omits `refresh_token` on login | Fail login and suggest consent/client configuration checks. |
| Cached ID token expired and refresh succeeds | Store the new ID token metadata and continue. |
| Refresh token revoked or rejected | Delete or mark the local session unusable and tell the user to run `sqlrs auth login google`. |
| Gateway rejects ID token with `401` | Surface the API auth error; audience/issuer troubleshooting belongs in the auth guide. |

## 8. Security Invariants

- Refresh token-ы никогда не покидают client machine, кроме запросов к Google
  token или revocation endpoint.
- sqlrs gateway никогда не получает refresh token-ы.
- Workspace config хранит только non-secret auth configuration.
- Raw refresh token-ы и raw ID token-ы никогда не печатаются в normal, JSON или
  verbose output.
- Loopback listener bind-ится только к `127.0.0.1` и принимает один callback
  для одной login attempt.
- `state` и `nonce` high entropy и single-use.
- PKCE использует `S256`.

## 9. References

- User guide: [`../user-guides/sqlrs-auth.md`](../user-guides/sqlrs-auth.md)
- ADR: [`../adr/2026-07-01-google-oidc-cli-auth.md`](../adr/2026-07-01-google-oidc-cli-auth.md)
- CLI contract: [`cli-contract.RU.md`](cli-contract.RU.md)
- CLI architecture: [`cli-architecture.RU.md`](cli-architecture.RU.md)
- CLI auth component structure:
  [`cli-auth-component-structure.RU.md`](cli-auth-component-structure.RU.md)
- User/org flow: [`user-org-flow.RU.md`](user-org-flow.RU.md)
