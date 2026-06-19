# Using Pup with Leash

[Pup](https://github.com/DataDog/pup) is Datadog's official CLI. This guide covers what pup can and cannot do for Leash credential management.

**What pup is useful for with Leash:**
- Verifying that your API/App keys have the correct scopes before running Leash
- Rotating keys once you have an initial set
- Listing and inspecting existing keys

**What pup cannot do (v1.2.1):** `api_keys_write` and `application_keys_write` are not in pup's OAuth client registration, so `api-keys create` and `app-keys create` return `403 Forbidden` even after a successful `pup auth login`. **Create your initial keys via the [Datadog web UI](https://app.datadoghq.com/organization-settings/api-keys)** — see [auth.md](auth.md) for the step-by-step.

---

For installation instructions see the [pup repository](https://github.com/DataDog/pup).

---

## Authenticate pup

Pup uses **OAuth2 + PKCE** — no long-lived keys needed to operate pup itself.

```bash
# Log in (opens a browser tab)
pup auth login

# EU region
pup auth login --site datadoghq.eu

# Check status
pup auth status
```

Tokens are stored in your OS keychain and automatically refreshed.

---

## Verify keys work

After creating your API and App keys (via the web UI or another method), use pup to confirm they have the right scopes. Pass the keys as environment variables:

```bash
# Spot-check: list monitors (requires monitors_read)
DD_API_KEY=$DD_API_KEY DD_APP_KEY=$DD_APP_KEY pup monitors list --limit 1

# Spot-check: list SLOs (requires slos_read)
DD_API_KEY=$DD_API_KEY DD_APP_KEY=$DD_APP_KEY pup slos list --limit 1

# Spot-check: list synthetics (requires synthetics_read)
DD_API_KEY=$DD_API_KEY DD_APP_KEY=$DD_APP_KEY pup synthetics list --limit 1

# Spot-check: list users (requires user_access_read)
DD_API_KEY=$DD_API_KEY DD_APP_KEY=$DD_APP_KEY pup users list --limit 1
```

If any command returns `403 Forbidden`, the App key is missing that scope. Add it in the Datadog web UI under **Organization Settings → Application Keys → Edit**.

---

## Key rotation with pup

Once you have an initial `DD_API_KEY` + `DD_APP_KEY`, pup can create replacement keys for rotation:

```bash
export DD_API_KEY=<current-api-key>
export DD_APP_KEY=<current-app-key>

# 1. Create replacement keys
NEW_API=$(pup --no-agent api-keys create --name leash-production-v2 | jq -r '.data.attributes.key')
NEW_APP=$(pup --no-agent app-keys create --name leash-readonly-v2 \
  --scopes "monitors_read,slos_read,synthetics_read,dashboards_read,user_access_read" \
  | jq -r '.data.attributes.key')

# 2. Update your secret store / .env with the new values

# 3. Verify the new keys work
DD_API_KEY=$NEW_API DD_APP_KEY=$NEW_APP pup monitors list --limit 1

# 4. List old keys to find their IDs
pup --no-agent api-keys list | jq '.data[] | {id: .id, name: .attributes.name}'
pup --no-agent app-keys list | jq '.data[] | {id: .id, name: .attributes.name}'

# 5. Delete the old keys
pup api-keys delete <old-api-key-id>
pup app-keys delete <old-app-key-id>
```

> **`--no-agent` in scripts:** When pup runs inside an AI coding assistant (Claude Code, Cursor, etc.) it wraps JSON in a `{status, data, metadata}` envelope. In a normal shell session it emits raw JSON. Scripts must include `--no-agent` so jq paths are stable regardless of where the script runs.

---

## Multi-site and multi-org

```bash
# Log in to each org with a named session
pup auth login --org prod
pup auth login --org staging
pup auth login --site datadoghq.eu --org eu-prod

# Run commands against a specific org
pup --org staging monitors list
DD_ORG=eu-prod pup slos list

# Key creation still requires DD_API_KEY + DD_APP_KEY for that org
DD_API_KEY=<org-key> DD_APP_KEY=<org-key> DD_ORG=staging \
  pup api-keys create --name leash-staging
```

---

## Scope reference for Leash App keys

| Scope | Required for |
|---|---|
| `monitors_read` | `datadog.monitor` resource (listing, filtering) |
| `monitors_write` | `tag` and `delete` actions on monitors |
| `slos_read` | `datadog.slo` resource |
| `slos_write` | `tag` action on SLOs |
| `synthetics_read` | `datadog.synthetic` resource |
| `dashboards_read` | `datadog.dashboard` resource |
| `user_access_read` | `datadog.user` resource |
| `user_access_manage` | `delete` (disable) action on users |

**Minimum read-only set** (dry-run / report-only policies):
```
monitors_read,slos_read,synthetics_read,dashboards_read,user_access_read
```

**Full set** (includes mutating actions):
```
monitors_read,monitors_write,slos_read,slos_write,synthetics_read,dashboards_read,user_access_read,user_access_manage
```

---

## pup command reference (key management)

All key management commands require `DD_API_KEY` + `DD_APP_KEY` to be set — OAuth is not sufficient.

| Command | Description |
|---|---|
| `pup api-keys list` | List API keys in the org |
| `pup api-keys get <id>` | Get metadata for a specific API key |
| `pup api-keys create --name <name>` | Create a new API key |
| `pup api-keys delete <id>` | Delete an API key |
| `pup app-keys list` | List App keys for the current user |
| `pup app-keys list --all` | List all App keys in the org |
| `pup app-keys get <id>` | Get metadata for a specific App key |
| `pup app-keys create --name <name> [--scopes ...]` | Create a new App key |
| `pup app-keys update <id> [--name ...] [--scopes ...]` | Update an App key's name or scopes |
| `pup app-keys delete <id>` | Delete an App key |

Global flags:

| Flag | Description |
|---|---|
| `--output json` | Output raw JSON (the default) |
| `--output yaml` | Output YAML |
| `--org <name>` | Use a named pup session (multi-org) |
| `--site <host>` | Override Datadog site |
| `--no-agent` | Emit raw JSON; required in scripts |
