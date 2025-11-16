# Drone 1Password Secret Plugin

This service resolves Drone secrets from 1Password Connect by vault and item name. Each request is authenticated with a bearer token and executed live against the Connect REST API described in `1password-connect-api_1.8.1.yaml`.

## Requirements

- Drone server and runners version 1.4 or newer
- A reachable 1Password Connect deployment
- A Connect token with read access to the relevant vaults and items

## Known issues

- Name is currently not used but still enforced.
- Names/Vauls with `/` may not work.

## Environment variables

| Variable | Description |
| --- | --- |
| `DRONE_SECRET` | Shared secret used to authenticate Drone requests. |
| `DRONE_BIND` | Address for the HTTP listener (default `:3000`). |
| `DRONE_DEBUG` | Set to `true` for debug logging. |
| `OP_CONNECT_HOST` | Base URL for 1Password Connect (for example `https://connect.example.com`). The server automatically targets the `/v1` API. |
| `OP_CONNECT_TOKEN` | Bearer token for the Connect API. |
| `OP_CONNECT_TIMEOUT` | Optional HTTP timeout such as `20s` (default `15s`). |

Expose the plugin to Drone runners:

```text
DRONE_SECRET_PLUGIN_ENDPOINT=http://1.2.3.4:3000
DRONE_SECRET_PLUGIN_TOKEN=${DRONE_SECRET}
```

### Docker example

```bash
docker run -d \
  --name drone-1password \
  --restart always \
  -p 3000:3000 \
  -e DRONE_SECRET=${DRONE_SECRET} \
  -e OP_CONNECT_HOST=https://connect.example.com \
  -e OP_CONNECT_TOKEN=${OP_CONNECT_TOKEN} \
  ghcr.io/m42e/drone-1password:main
```

## Requesting secrets in pipelines

Set the secret `path` to `vault/item[/field]`:

- Segment one: vault name (case-sensitive match against Connect).
- Segment two: item title inside that vault.
- Optional segment three: field selector. When omitted, the plugin returns the first password field.

Field selectors behave as follows:

- Unqualified labels such as `Password` resolve when only one matching field exists.
- Section-qualified labels use `Section Name / Field Label` (for example `Database Credentials / Token`).
- The special selector `notes` (or `notesPlain`) returns the item notes content.
- Ambiguous labels cause the request to fail with guidance to supply a section-qualified selector.

Example `.drone.yml` fragment:

```yaml
secrets:
- name: db_password
  get:
    path: Production Vault/Database Credentials
    name: <ignored for now but required>
- name: api_token
  get:
    path: Production Vault/API Service/Service Keys / Token
    name: <ignored for now but required>
```

```jsonnet
local secret = {
  kind: "secret",
  name: "password",
  get: {
    path: "Dienste/Drone - Test - One",
    name: "Password",
  },
};
```

The first secret resolves the password automatically, while the second fetches the `Token` field within the `Service Keys` section.

## Failure modes

- Missing vaults, items, or fields: the plugin surfaces an error so the build fails fast.
- Ambiguous matches: errors direct you to use a section-qualified label.
- 1Password Connect HTTP errors: response codes and messages are preserved to simplify troubleshooting.

## Development notes

- No caching is performed; each lookup hits 1Password Connect.
- Tests rely on `httptest.Server` to emulate the Connect API.
