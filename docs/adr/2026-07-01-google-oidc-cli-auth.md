# 2026-07-01 Google OIDC CLI Auth

- Conversation timestamp: 2026-07-01T17:43:30.5482893+07:00
- GitHub user id: @evilguest
- Agent name/version: Codex / GPT-5
- Status: Accepted

## Decision Record 1: use Google Authorization Code with PKCE over loopback

### Question discussed

How should the CLI obtain and maintain Google OIDC authentication for protected
remote sqlrs API commands without requiring users to paste a short-lived ID
token every hour?

### Alternatives considered

1. Keep the existing `SQLRS_TOKEN` model only. Users manually provide a
   short-lived Google ID token as an environment variable.
2. Use Google OAuth/OIDC Authorization Code Flow with PKCE and a loopback
   redirect listener on `127.0.0.1:<random-port>`.
3. Use Google device authorization flow.
4. Send refresh tokens to the sqlrs gateway and let the server refresh ID
   tokens.

### Chosen solution

Adopt option 2 as the primary CLI login flow.

The CLI adds:

```text
sqlrs auth login google
sqlrs auth status
sqlrs auth logout
```

`sqlrs auth login google` uses Google Authorization Code Flow with PKCE,
`openid email profile` scopes, `access_type=offline`, and `prompt=consent`. It
starts a loopback listener on `127.0.0.1:<random-port>`, opens the Google
authorization URL in the system browser, validates `state` and OIDC `nonce`,
exchanges the authorization code at Google's token endpoint, and stores the
refresh token locally.

Protected remote API commands keep `SQLRS_TOKEN` as the highest-priority
override. If `SQLRS_TOKEN` is not set and the selected remote profile uses
`auth.mode: oidcSession`, the CLI obtains a fresh Google ID token from the
local session and sends only that ID token as the API bearer token.

The gateway continues to accept only short-lived Google ID tokens. Refresh
tokens are never sent to the gateway.

### Brief rationale

The loopback Authorization Code with PKCE flow matches a workstation CLI with a
browser, avoids embedding a client secret, and lets the CLI obtain a refresh
token without exposing it to the server. Keeping `SQLRS_TOKEN` as an override
preserves the existing smoke/debug workflow while making normal user sessions
long-lived and automatic.

Device flow is reserved as a fallback because it is intended for limited-input
devices and must be rechecked for refresh-token behavior with the selected
Google OAuth client type before use.

## Decision Record 2: store refresh tokens only in OS credential storage

### Question discussed

Where should the CLI persist Google refresh tokens and auth-session metadata?

### Alternatives considered

1. Store refresh tokens in `.sqlrs/config.yaml`.
2. Store refresh tokens in a sqlrs-owned plaintext file under the user's config
   directory.
3. Store refresh tokens only in the operating-system credential store:
   Windows Credential Manager, macOS Keychain, or Linux Secret Service/libsecret.
4. Store refresh tokens on the sqlrs gateway.

### Chosen solution

Adopt option 3.

Refresh tokens live only in the OS credential store. The workspace or global
sqlrs config may store non-secret profile metadata such as provider, issuer,
client ID, endpoint, profile name, account email, and token expiry, but it must
not store raw refresh tokens or raw ID tokens.

If Linux Secret Service/libsecret is unavailable, login fails with a clear
credential-store setup error. There is no plaintext refresh-token fallback.

### Brief rationale

Refresh tokens are durable bearer secrets. Storing them in OS credential storage
uses platform-native protection and keeps the workspace config safe to inspect
or commit accidentally. Keeping refresh tokens out of the gateway preserves the
server boundary: server-side auth accepts only short-lived ID tokens.

## Related documents

- `docs/user-guides/sqlrs-auth.md`
- `docs/user-guides/sqlrs-users-orgs.md`
- `docs/adr/2026-06-20-user-registration-command-split.md`
- `docs/adr/2026-06-20-user-identity-conditional-put.md`

## Contradiction check

No existing ADR was marked obsolete. Existing user and organization ADRs assume
that the gateway receives a validated bearer token and do not define how the CLI
obtains that token. This ADR supplies that missing CLI login path without
changing user or organization API semantics.
