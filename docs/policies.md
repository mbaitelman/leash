# Policy syntax reference

Policies are YAML files containing a top-level `policies` list. Each entry selects a resource type, optionally narrows the set with filters, and executes actions on every match.

```yaml
policies:
  - name: string           # required — unique identifier used in reports
    description: string    # optional — shown in the UI and findings output
    resource: string       # required — see Resource types
    params: {}             # optional — provider-specific parameters (see Provider parameters)
    filters: []            # optional — all filters AND-ed; no filters = match everything
    actions: []            # optional — executed in order on each matched resource
```

---

## Resource types

| Name | Datadog entity | Taggable | Tag-removable | Deletable |
|---|---|---|---|---|
| `datadog.monitor` | Monitors (metric, log, APM, composite, …) | yes | yes | yes |
| `datadog.slo` | Service Level Objectives | yes | yes | no |
| `datadog.synthetic` | Synthetic API and browser tests | no | no | no |
| `datadog.dashboard` | Dashboards | no | no | no |
| `datadog.user` | User accounts | no | no | yes (disables) |
| `datadog.rum_application` | RUM applications | no | no | no |
| `datadog.rum_retention_filter` | RUM retention filters (one resource per filter per app) | no | no | no |
| `datadog.audit_event` | Audit Trail events (windowed search) | no | no | no |

> **Note on deletion:** Datadog does not support hard-deleting user accounts via API. The `delete` action on `datadog.user` calls `DisableUser`, not a destructive delete. The `Taggable` / `Tag-removable` / `Deletable` columns determine which actions are available — using `tag` or `delete` on a resource type that doesn't support it produces a runtime error recorded in the action log.

---

## Provider parameters (`params`)

Some resource types accept per-policy parameters via an optional `params` block. Using `params` on a resource type that doesn't accept them is a validation error.

Currently only `datadog.audit_event` accepts parameters:

| Key | Type | Default | Description |
|---|---|---|---|
| `query` | string | — | Server-side [Audit Logs search query](https://docs.datadoghq.com/account_management/audit_trail/) applied by the Datadog API before events are fetched, e.g. `"@evt.name:Dashboard @action:deleted"` |
| `lookback` | duration string | `24h` | Search window ending at `to`. Supports Go durations plus `d` for days (`24h`, `7d`). Mutually exclusive with `from` |
| `from` | string | — | Window start: RFC3339 timestamp or Datadog date math (`now-24h`) |
| `to` | string | `now` | Window end: RFC3339 timestamp or Datadog date math |
| `max_events` | int | `1000` | Hard cap on the number of events fetched per run |

```yaml
policies:
  - name: audit-example
    resource: datadog.audit_event
    params:
      query: "@evt.name:Dashboard @action:deleted"
      lookback: 24h
      max_events: 500
```

`leash validate` checks `params` offline — unknown keys, bad durations, and conflicting `lookback`/`from` are rejected without API calls.

---

## Filters

Filters in a policy's `filters` list are AND-ed together: a resource must pass every filter to be considered a match. Use boolean meta-filters (`and`, `or`, `not`) to build more complex logic.

If `filters` is omitted or empty, every resource of the given type matches.

---

### `value` filter

Compares any field in the resource's property map.

```yaml
- type: value
  key: name           # required — property key (dot-notation for nested fields)
  op: regex           # required — see ops table below
  value: "^\\[PROD\\]"  # required for most ops; omit for present/absent
```

**Ops:**

| Op | Description |
|---|---|
| `present` | Field exists and is non-nil |
| `absent` | Field does not exist or is nil |
| `eq` | String equality (`fmt.Sprintf("%v", v)` comparison) |
| `ne` | String inequality |
| `contains` | String contains substring, OR array/slice contains element |
| `not-contains` | Inverse of `contains` |
| `regex` | Go regular expression match on stringified value |
| `not-regex` | Inverse of `regex` |
| `in` | Target value is a member of a YAML list |
| `not-in` | Target value is not in the list |
| `gt` / `lt` | Numeric greater-than / less-than |
| `gte` / `lte` | Numeric greater-than-or-equal / less-than-or-equal |

**Examples:**

```yaml
# Absent field (works for computed properties like synthetic.slo_linked)
- type: value
  key: synthetic.slo_linked
  op: absent

# Array membership — flag resources with a specific state
- type: value
  key: overall_state
  op: in
  value: ["Alert", "Warn", "No Data"]

# Array contains — tags array contains a substring match
- type: value
  key: tags
  op: contains
  value: "env:prod"

# Numeric threshold
- type: value
  key: options.thresholds.critical
  op: gt
  value: 1000
```

---

### `tag` filter

Convenience filter for Datadog tag arrays (`key:value` strings). More readable than `value: {key: tags, op: contains}` for common tag checks.

```yaml
- type: tag
  key: env            # required — the key part of the tag (before ":")
  value: prod         # optional — the value part; omit to check for any value
  op: present         # optional — present (default) | absent | eq
```

**How matching works:**

- Without `value`: matches any tag equal to `key` OR starting with `key:` (any value).
- With `value`: matches the tag `key:value` exactly.
- `op: present` (default) — match if such a tag exists.
- `op: absent` — match if no such tag exists.
- `op: eq` — same as `present` with a `value`; explicit form.

**Examples:**

```yaml
# Resource has any env tag (env:prod, env:staging, env, …)
- type: tag
  key: env

# Resource has env:prod specifically
- type: tag
  key: env
  value: prod

# Resource is missing a team tag entirely
- type: tag
  key: team
  op: absent

# Resource does NOT have env:prod (has other envs or no env tag)
- type: tag
  key: env
  value: prod
  op: absent
```

---

### `age` filter

Filters resources based on the age of a timestamp field.

```yaml
- type: age
  key: modified       # required — must be a time.Time field
  op: older-than      # required — older-than | newer-than
  value: "90d"        # required — duration string
```

**Duration format:** Standard Go duration strings (`24h`, `168h`, `30m`) plus a `d` shorthand for days (`7d` = 7 days, `90d` = 90 days). Days are always 24-hour periods.

If the field is absent or null on a resource, the filter does not match (returns false).

**Examples:**

```yaml
# SLO not modified in 90 days
- type: age
  key: modified
  op: older-than
  value: "90d"

# User account created in the last 7 days
- type: age
  key: created
  op: newer-than
  value: "7d"
```

---

### Boolean meta-filters

Combine other filters with `and`, `or`, or `not`. Meta-filters can be nested arbitrarily.

**`and`** — all children must match (same as a flat list, but useful inside `or`):

```yaml
- and:
    - type: tag
      key: env
      value: prod
    - type: tag
      key: team
      op: absent
```

**`or`** — any child must match:

```yaml
- or:
    - type: tag
      key: env
      value: prod
    - type: tag
      key: env
      value: staging
```

**`not`** — inverts exactly one child filter:

```yaml
- not:
    - type: value
      key: name
      op: regex
      value: "^\\[REVIEWED\\]"
```

> `not` requires exactly one child. Wrapping multiple conditions in `not` is a validation error — use `not` around an `and` or `or` group instead.

**Nested example:**

```yaml
filters:
  # Must be a prod or staging env
  - or:
      - type: tag
        key: env
        value: prod
      - type: tag
        key: env
        value: staging
  # AND must not have a team tag
  - type: tag
    key: team
    op: absent
  # AND must not have been reviewed recently
  - not:
      - type: age
        key: modified
        op: newer-than
        value: "30d"
```

---

## Actions

Actions are executed in declaration order on every matched resource. Each action records a result (success/failure, dry-run flag) in the `actions_taken` array of the findings report.

All mutating actions (`tag`, `delete`, `notify`) respect `--dry-run`: in dry-run mode they log what they would do but make no API calls or HTTP requests.

---

### `report`

Records the match to stdout and the structured log. Always runs, including in dry-run mode.

```yaml
- type: report
```

No fields. Including `report` is optional but recommended — it ensures matches appear in terminal output during `leash run`, independent of the findings JSON file.

---

### `tag`

Adds one or more Datadog tags to the matched resource. Optionally removes those same tags from resources that *pass* the policy (i.e., are now compliant).

```yaml
- type: tag
  tags:
    - "compliance:violation"
    - "leash:flagged"
  remove_on_pass: false   # optional — default false
```

**Fields:**

| Field | Required | Default | Description |
|---|---|---|---|
| `tags` | Yes | — | List of `key:value` tag strings to add to matched resources |
| `remove_on_pass` | No | `false` | When `true`, also removes the listed tags from resources that pass the policy |

**Behavior:**
- **Additive only (default)** — existing tags are never removed. Tags already present on the resource are silently skipped.
- **Idempotent** — running the same policy twice does not duplicate tags.
- Supported for add: `datadog.monitor`, `datadog.slo`. Using `tag` on other resource types records an error in `actions_taken`.

**`remove_on_pass`:**

Set `remove_on_pass: true` to automatically clean up leash-applied marker tags once a resource becomes compliant again:

```yaml
- type: tag
  tags:
    - "leash:missing-team"
  remove_on_pass: true
```

- Resources that **match** (still non-compliant) → tags added as normal.
- Resources that **pass** (now compliant) → listed tags removed if present; tags already absent are silently skipped.
- Dry-run is fully respected — no API call is made.
- Supported resources (tag removal): `datadog.monitor`, `datadog.slo`. Using `remove_on_pass` on other resource types records an error in the server log.

---

### `notify`

POSTs a message to a Slack incoming webhook.

```yaml
- type: notify
  channel: slack
  webhook_url: "https://hooks.slack.com/services/T.../B.../..."
  # Falls back to SLACK_WEBHOOK_URL environment variable if omitted
```

**Fields:**

| Field | Required | Description |
|---|---|---|
| `channel` | Yes | Must be `slack` (only Slack is currently supported) |
| `webhook_url` | No | Webhook URL; falls back to `SLACK_WEBHOOK_URL` env var |

If neither `webhook_url` nor the env var is set, the action records an error but does not abort the run.

---

### `delete`

Deletes or disables the matched resource. Requires two explicit opt-ins to prevent accidental use.

```yaml
- type: delete
  confirm: true       # Must be present in the YAML
  # AND: leash run --dry-run=false must be passed on the CLI
```

**Per-resource behavior:**

| Resource | What happens |
|---|---|
| `datadog.monitor` | Hard-deletes the monitor via the Datadog API |
| `datadog.user` | Disables the user account (`DisableUser` API) — Datadog does not support hard user deletion |

Using `delete` on `datadog.slo`, `datadog.synthetic`, `datadog.dashboard`, `datadog.rum_application`, `datadog.rum_retention_filter`, or `datadog.audit_event` records an error (not implemented).

---

## Resource field reference

These are the property keys available for `value` and `age` filters. The `tag` filter always reads the `tags` field regardless of what is listed here.

### `datadog.monitor`

| Key | Type | Description |
|---|---|---|
| `id` | int64 | Monitor ID |
| `name` | string | Monitor name |
| `message` | string | Notification message body |
| `query` | string | Monitor query |
| `type` | string | `metric alert`, `log alert`, `service check`, `event alert`, etc. |
| `tags` | []string | Tag list |
| `created` | time.Time | Creation timestamp |
| `modified` | time.Time | Last modified timestamp |
| `overall_state` | string | `OK`, `Alert`, `Warn`, `No Data`, `Unknown`, `Ignored` |
| `creator.email` | string | Email of the user who created the monitor |
| `creator.handle` | string | Datadog handle of the creator |
| `options.notify_no_data` | bool | Whether no-data state triggers an alert |
| `options.require_full_window` | bool | Whether the monitor requires a full evaluation window |
| `options.thresholds.critical` | float64 | Critical alert threshold value |

### `datadog.slo`

| Key | Type | Description |
|---|---|---|
| `id` | string | SLO ID |
| `name` | string | SLO name |
| `description` | string | Description |
| `type` | string | `metric`, `monitor`, or `time_slice` |
| `tags` | []string | Tag list |
| `created` | time.Time | Creation timestamp |
| `modified` | time.Time | Last modified timestamp |
| `creator.email` | string | Creator's email address |

### `datadog.synthetic`

| Key | Type | Description |
|---|---|---|
| `public_id` | string | Test public ID (e.g. `abc-123-def`) |
| `name` | string | Test name |
| `type` | string | `api` or `browser` |
| `status` | string | `live` or `paused` |
| `tags` | []string | Tag list |
| `creator.email` | string | Creator's email address |
| `monitor_id` | int64 | ID of the monitor Datadog auto-creates for this test |
| `synthetic.slo_linked` | bool | `true` if the test's monitor is referenced by at least one SLO; **absent** when not linked — use `op: absent` to find unlinked tests |

### `datadog.dashboard`

> **Performance note:** The Datadog list-dashboards endpoint does not return tags. Leash fetches each dashboard individually to populate the `tags` field. For organizations with many dashboards this scan will be slower than other resource types.

| Key | Type | Description |
|---|---|---|
| `id` | string | Dashboard ID |
| `title` | string | Dashboard title |
| `author_handle` | string | Author's email or handle |
| `creator.email` | string | Same as `author_handle`; use this for cross-resource consistency |
| `description` | string | Description |
| `layout_type` | string | `ordered` or `free` |
| `url` | string | Relative URL path (e.g. `/dashboard/abc-123`) |
| `tags` | []string | Tag list |
| `created` | time.Time | Creation timestamp |
| `modified` | time.Time | Last modified timestamp |

### `datadog.user`

| Key | Type | Description |
|---|---|---|
| `id` | string | User ID |
| `email` | string | Email address |
| `name` | string | Display name |
| `title` | string | Job title |
| `status` | string | `Active`, `Pending`, or `Disabled` |
| `disabled` | bool | Whether the account is currently disabled |
| `service_account` | bool | Whether this is a service account |
| `created` | time.Time | Account creation timestamp |
| `modified` | time.Time | Last modified timestamp |

### `datadog.rum_application`

| Key | Type | Description |
|---|---|---|
| `id` | string | Application ID |
| `name` | string | Application name |
| `type` | string | `browser`, `ios`, `android`, `react-native`, `flutter`, `roku`, `electron`, `unity`, `kotlin-multiplatform` |
| `is_active` | bool | Whether the application is active and collecting data |
| `creator.email` | string | Handle of the user who created the application |
| `created` | time.Time | Creation timestamp |
| `updated` | time.Time | Last updated timestamp |
| `updated_by_handle` | string | Handle of the user who last updated the application |
| `product_scales.rum_processing_state` | string | `ALL`, `ERROR_FOCUSED_MODE`, or `NONE` — controls which RUM events are processed |
| `product_scales.analytics_retention_state` | string | `MAX` or `NONE` — controls Product Analytics data retention |

### `datadog.rum_retention_filter`

Each retention filter for a RUM application is a separate resource. Use `app_id` to correlate back to `datadog.rum_application`.

| Key | Type | Description |
|---|---|---|
| `id` | string | Filter ID |
| `app_id` | string | Parent RUM application ID |
| `app_name` | string | Parent RUM application name |
| `name` | string | Filter name |
| `enabled` | bool | Whether the filter is active |
| `event_type` | string | `session`, `view`, `action`, `error`, `resource`, or `long_task` |
| `query` | string | RUM search query scoping this filter |
| `sample_rate` | float64 | Sampling rate (0.1–100) |

### `datadog.audit_event`

One resource per Audit Trail event in the search window (see [Provider parameters](#provider-parameters-params)). Events are immutable — `tag` and `delete` record errors; use `report` and `notify`.

| Key | Type | Description |
|---|---|---|
| `id` | string | Unique event ID |
| `message` | string | Event message |
| `service` | string | Application or service that generated the event |
| `tags` | []string | Event tags |
| `timestamp` | time.Time | Event timestamp (works with the `age` filter) |
| `attributes.*` | any | All event attributes, dot-flattened dynamically |

The `attributes.*` keys mirror the Audit Trail facets with the `@` prefix replaced by `attributes.`: `@evt.name` becomes `attributes.evt.name`, `@usr.email` becomes `attributes.usr.email`, `@action` becomes `attributes.action`, and so on. Available keys vary by event type.

> **Scheduled runs:** each run re-fetches the window independently — there is no deduplication across runs. Set `lookback` to at least the `serve --schedule` interval to avoid gaps, and expect `notify` to fire again for events that appear in overlapping windows.

---

## Complete examples

### Auto-clean marker tags when a resource becomes compliant

`remove_on_pass: true` closes the feedback loop: leash adds the tag when a resource is non-compliant and removes it once the resource passes again — with no manual cleanup required.

```yaml
policies:
  - name: slo-missing-team-tag
    description: SLOs must declare an owning team. Tags are added on violation and removed on remediation.
    resource: datadog.slo
    filters:
      - type: tag
        key: team
        op: absent
    actions:
      - type: report
      - type: tag
        tags:
          - "leash:missing-team"
        remove_on_pass: true   # removes "leash:missing-team" once the SLO has a team tag
```

---

### Flag production monitors missing a team tag

```yaml
policies:
  - name: prod-monitors-need-team-tag
    description: Every production monitor must declare an owning team.
    resource: datadog.monitor
    filters:
      - type: tag
        key: env
        value: prod
      - type: tag
        key: team
        op: absent
    actions:
      - type: report
      - type: tag
        tags:
          - "leash:missing-team"
      - type: notify
        channel: slack
```

### Require synthetics to have a linked SLO

```yaml
policies:
  - name: synthetic-missing-slo
    description: All synthetic tests must be backed by a monitor-based SLO.
    resource: datadog.synthetic
    filters:
      - type: value
        key: synthetic.slo_linked
        op: absent
    actions:
      - type: report
      - type: tag
        tags:
          - "leash:missing-slo"
```

### Stale SLOs in prod or staging

```yaml
policies:
  - name: stale-prod-slos
    description: SLOs in prod or staging not updated in 90 days need a review.
    resource: datadog.slo
    filters:
      - or:
          - type: tag
            key: env
            value: prod
          - type: tag
            key: env
            value: staging
      - type: age
        key: modified
        op: older-than
        value: "90d"
    actions:
      - type: report
      - type: tag
        tags:
          - "leash:stale"
```

### Disable inactive user accounts

```yaml
policies:
  - name: disable-pending-users
    description: Users who never accepted their invite (Pending) for over 30 days.
    resource: datadog.user
    filters:
      - type: value
        key: status
        op: eq
        value: Pending
      - type: age
        key: created
        op: older-than
        value: "30d"
    actions:
      - type: report
      - type: delete
        confirm: true
```
