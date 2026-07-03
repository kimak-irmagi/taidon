# sqlrs auth

## Overview

`sqlrs auth` manages local CLI authentication sessions for shared or cloud
deployments. The first supported provider is Google OIDC.

The server and gateway continue to accept only short-lived Google ID tokens as
`Authorization: Bearer <id-token>`. Refresh tokens are client-only secrets. The
CLI stores them in the operating-system credential store, refreshes ID tokens
locally, and never sends refresh tokens to any sqlrs API.

Manual bearer-token use through `SQLRS_TOKEN` remains a debug and smoke-test
override. If `SQLRS_TOKEN` is set, protected API commands use it before any
stored auth session.

## Command syntax

```text
sqlrs auth login google [--login-hint <email>] [--no-browser]
sqlrs auth status
sqlrs auth logout [--no-revoke]
```

Global flags continue to work:

```text
sqlrs --profile remote-dev auth login google
sqlrs --profile remote-dev auth status
sqlrs --profile remote-dev --output json auth status
sqlrs --profile remote-dev auth logout
```

`auth` is a remote-profile command group. Local profiles do not need Google
login because local engine requests use the local daemon token from
`engine.json`.

## Remote profile configuration

A Google-backed remote profile stores the OAuth client metadata needed to start
and refresh the Google session:

```yaml
profiles:
  remote-dev:
    mode: remote
    endpoint: "https://sqlrs.example.org"
    auth:
      mode: oidcSession
      clientID: "1234567890-abcdef.apps.googleusercontent.com"
      clientSecret: "google-desktop-client-secret"
      issuer: "https://accounts.google.com"
      tokenEnv: SQLRS_TOKEN
```

Rules:

- `auth.mode: oidcSession` tells protected remote API commands to use a stored,
  refresh-capable OIDC session instead of a static bearer token.
- There is no separate `auth.provider` field in this slice. The provider is
  selected by `sqlrs auth login google` and then recorded in the local session
  metadata.
- `auth.clientID` is the Google OAuth client ID and is also the `aud` value the
  gateway must accept.
- `auth.clientSecret` is a temporary compatibility field for Google Desktop
  OAuth clients that require `client_secret` at the token endpoint. The CLI sends
  it only to Google during authorization-code and refresh-token exchanges. It is
  not sent to the sqlrs gateway and must not be treated as a gateway trust
  boundary.
- `auth.issuer` defaults to `https://accounts.google.com` when omitted.
- `auth.tokenEnv` defaults to `SQLRS_TOKEN`; when that environment variable is
  set, it overrides the stored Google session.
- Existing `auth.mode: bearer` profiles keep their current behavior and read a
  caller-supplied bearer token from `tokenEnv` or `token`.

Do not store refresh tokens or raw ID tokens in `.sqlrs/config.yaml`. The
temporary `auth.clientSecret` value should live in user-local config rather than
in a committed workspace config until a better secret source is designed.

## Google OAuth client

Create a Google OAuth client for the CLI:

1. In Google Cloud Console, create an OAuth client with application type
   `Desktop app`.
2. Put the resulting client ID into `profiles.<name>.auth.clientID`.
3. If the downloaded Desktop client JSON contains `installed.client_secret`, put
   it into `profiles.<name>.auth.clientSecret` for now.
4. Configure the sqlrs gateway to accept Google ID tokens with:
   - issuer: `https://accounts.google.com`;
   - audience: the same Google client ID.

The CLI uses a loopback redirect URI at runtime:

```text
http://127.0.0.1:<random-port>
```

The random port is selected for each login. Do not configure a fixed redirect
port in sqlrs config. If the gateway currently supports only one accepted
audience and this CLI client ID differs from the existing frontend or service
client ID, create a separate gateway task to support multiple accepted
audiences.

When `auth.clientSecret` is configured, the CLI sends it only to the Google
token endpoint. The gateway still validates only Google ID token claims and must
not rely on the Desktop client secret.

## `sqlrs auth login google`

Logs the selected remote profile into Google and stores a refresh-capable local
session.

```text
sqlrs auth login google [--login-hint <email>] [--no-browser]
```

Flow:

1. Validate that the selected profile is `mode: remote` and
   `auth.mode: oidcSession`.
2. Generate:
   - a high-entropy PKCE verifier;
   - a SHA-256 PKCE challenge using method `S256`;
   - a high-entropy `state`;
   - a high-entropy OIDC `nonce`.
3. Start an HTTP listener on `127.0.0.1:<random-port>`.
4. Build the Google authorization URL with:
   - `response_type=code`;
   - `client_id=<profile auth.clientID>`;
   - `redirect_uri=http://127.0.0.1:<random-port>`;
   - `scope=openid email profile`;
   - `code_challenge=<challenge>`;
   - `code_challenge_method=S256`;
   - `state=<state>`;
   - `nonce=<nonce>`;
   - `access_type=offline`;
   - `prompt=consent`.
5. Open the system browser unless `--no-browser` is set. With `--no-browser`,
   print the authorization URL for the user to open manually.
6. Accept exactly one callback on the loopback listener.
7. Reject the callback if:
   - `state` does not match;
   - Google returns `error`;
   - `code` is missing.
8. Exchange the authorization code at the Google token endpoint using the PKCE
   verifier and `auth.clientSecret` when configured.
9. Require an ID token and refresh token in the token response.
10. Decode the ID token claims for local validation and metadata:
    - `iss` must match the configured issuer;
    - `aud` must match `auth.clientID`;
    - `exp` must be in the future;
    - `nonce` must match the login nonce;
    - `email` is captured for status output when present.
11. Store the refresh token and optional cached ID token in the OS credential
    store.
12. Store only safe metadata for status and diagnostics.

Human success output:

```text
logged in
provider: google
email: alice@example.com
issuer: https://accounts.google.com
audience: 1234567890-abcdef.apps.googleusercontent.com
profile: remote-dev
endpoint: https://sqlrs.example.org
```

The command never prints raw refresh tokens or raw ID tokens.

## Token selection for protected API commands

Protected remote API commands resolve a bearer token in this order:

1. If `SQLRS_TOKEN` is set, use it exactly as the bearer token source.
2. If the selected profile uses `auth.mode: oidcSession`, load the stored Google
   session from the OS credential store.
3. If the selected profile uses the legacy `auth.mode: bearer`, keep the current
   `tokenEnv` or `token` behavior.
4. If no token source is available, fail before making the protected API
   request.

For Google sessions, the CLI decodes the cached ID token locally and checks
`exp` before each protected API request. If the token is missing, expired, or
will expire within five minutes, the CLI refreshes it through the Google token
endpoint, including `auth.clientSecret` when configured, and then sends only the
fresh ID token as:

```text
Authorization: Bearer <id-token>
```

If refresh fails because the refresh token is missing, revoked, rejected, or the
credential store cannot be read, the CLI must not send the stale ID token. It
prints an actionable error:

```text
Google auth session is expired or unavailable; run `sqlrs auth login google`.
```

## `sqlrs auth status`

Shows whether the selected profile has a usable Google auth session.

```text
sqlrs auth status
```

Human output fields:

```text
status: logged in
provider: google
email: alice@example.com
issuer: https://accounts.google.com
audience: 1234567890-abcdef.apps.googleusercontent.com
tokenExpiry: 2026-07-01T12:45:00Z
profile: remote-dev
endpoint: https://sqlrs.example.org
override: none
```

If `SQLRS_TOKEN` is set, status must make the override visible:

```text
override: SQLRS_TOKEN
```

`auth status` never prints raw tokens. In verbose mode it may print only a safe
claim summary: `iss`, `aud`, masked `sub`, `email`, and `exp`.

JSON output uses the global `--output json` flag and must not include raw
tokens:

```json
{
  "status": "logged_in",
  "provider": "google",
  "email": "alice@example.com",
  "issuer": "https://accounts.google.com",
  "audience": "1234567890-abcdef.apps.googleusercontent.com",
  "token_expiry": "2026-07-01T12:45:00Z",
  "profile": "remote-dev",
  "endpoint": "https://sqlrs.example.org",
  "override": null
}
```

## `sqlrs auth logout`

Deletes the selected profile's local Google auth session.

```text
sqlrs auth logout [--no-revoke]
```

Default behavior:

1. Load the refresh token from the OS credential store.
2. Attempt to revoke it at the Google revocation endpoint.
3. Delete the local credential store entry and cached ID token even if
   revocation fails.
4. Leave `SQLRS_TOKEN` untouched.

`--no-revoke` skips the Google revocation request and only deletes local
credentials.

Human output:

```text
logged out
provider: google
profile: remote-dev
revoked: true
```

## Credential storage

The CLI stores refresh tokens only in the OS credential store:

- Windows: Windows Credential Manager.
- macOS: Keychain.
- Linux: Secret Service/libsecret.

The active-session credential lookup is scoped to:

- sqlrs application name;
- selected profile name;
- remote endpoint;
- provider;
- issuer;
- client ID.

The credential value may contain a JSON session record with:

- refresh token;
- optional cached ID token;
- token expiry;
- issuer;
- audience/client ID;
- subject as credential metadata;
- email;
- granted scopes;
- creation and update timestamps.

A successful `sqlrs auth login google` replaces the active Google session for
the selected profile and client ID. This is how a user switches Google accounts
for that profile.

If Linux Secret Service/libsecret is unavailable, `auth login google` fails with
a clear setup error. The CLI does not fall back to storing refresh tokens in
plain text. `SQLRS_TOKEN` remains available for smoke/debug runs.

## Troubleshooting

### Token expired

Protected commands refresh cached ID tokens automatically. If a command still
reports an expired or unavailable session, run:

```text
sqlrs auth login google
```

### Refresh token not returned

The login URL includes `access_type=offline` and `prompt=consent` so Google can
return a refresh token. If the token response still lacks `refresh_token`, check
that the OAuth client is a Desktop app client, the scopes are exactly
`openid email profile`, and the user completed consent for the selected Google
account.

### Google reports `client_secret is missing`

Some Google Desktop OAuth clients still require the downloaded
`installed.client_secret` during token endpoint calls even though Desktop
clients cannot keep that value confidential. Put it into
`profiles.<name>.auth.clientSecret` as a temporary compatibility setting and
retry login. The CLI sends this value only to Google `/token`; it is not sent to
the sqlrs gateway.

### Invalid audience

The ID token `aud` claim is the Google OAuth client ID used by the CLI. Configure
the gateway to accept that client ID. If the gateway supports only one accepted
audience today, add a gateway task for multiple accepted audiences before
rolling this CLI flow out.

### Credential store unavailable

On Linux, start or install a Secret Service provider such as GNOME Keyring or
KWallet with libsecret support. The CLI must not store refresh tokens in
`.sqlrs/config.yaml` as a fallback.

### Browser callback does not complete

Retry login. If the browser cannot be opened automatically, use:

```text
sqlrs auth login google --no-browser
```

Open the printed URL manually in a browser on the same machine where the CLI is
listening.

## Device code fallback

The first design target is the loopback redirect flow because sqlrs CLI runs on
developer workstations with a browser. Device code flow remains a fallback only
if loopback login proves impractical. Before switching, verify that the selected
Google OAuth client type and scopes return refresh tokens for device flow and
that the resulting ID token audience is accepted by the gateway.

## External references

- Google OAuth 2.0 for mobile and desktop apps:
  <https://developers.google.com/identity/protocols/oauth2/native-app>
- Google OAuth 2.0 for limited-input devices:
  <https://developers.google.com/identity/protocols/oauth2/limited-input-device>
