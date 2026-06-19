# Authentication

Leash authenticates to the Datadog API using an **API key** and an **Application key**. Both are required. Neither is a username/password — they are long-lived tokens scoped to your Datadog organization.

---

## Required credentials

| Environment variable | Description |
|---|---|
| `DD_API_KEY` | Identifies your Datadog organization. Used for every API call. |
| `DD_APP_KEY` | Grants specific permissions. Scopes what Leash can read and write. |
| `DD_SITE` | The Datadog regional endpoint. Defaults to `datadoghq.com`. |

Leash reads these automatically at startup via the Datadog Go SDK's `datadog.NewDefaultContext`. No code changes are needed to switch regions or rotate keys — update the environment variables and re-run.

---

## Creating API and Application keys

Create keys in the **Datadog web UI** — this is the only supported path for initial provisioning.

After you have keys, see [pup.md](pup.md) for using Datadog's official CLI to verify scope coverage and rotate keys.

### Datadog web UI

### API key

1. In Datadog, go to **Organization Settings → API Keys** (`https://app.datadoghq.com/organization-settings/api-keys`).
2. Click **New Key**.
3. Give it a descriptive name like `leash-production`.
4. Copy the key immediately — it is only shown once.

### Application key

1. Go to **Organization Settings → Application Keys** (`https://app.datadoghq.com/organization-settings/application-keys`).
2. Click **New Key**.
3. Name it `leash-production` (match the API key name for easy correlation).
4. **Scope the key** by selecting only the permissions Leash needs:

| Permission | Required for |
|---|---|
| `monitors_read` | `datadog.monitor` resource |
| `monitors_write` | `tag` and `delete` actions on monitors |
| `slos_read` | `datadog.slo` resource |
| `slos_write` | `tag` action on SLOs |
| `synthetics_read` | `datadog.synthetic` resource |
| `dashboards_read` | `datadog.dashboard` resource |
| `user_access_read` | `datadog.user` resource |
| `user_access_manage` | `delete` (disable) action on users |

**Principle of least privilege:** If you only run `report` and `notify` actions, you need only the `*_read` permissions. Grant write permissions only when you enable `--dry-run=false` with mutating actions.

---

## Regional sites

Set `DD_SITE` to the hostname for your Datadog region:

| Region | DD_SITE |
|---|---|
| US1 (default) | `datadoghq.com` |
| US3 | `us3.datadoghq.com` |
| US5 | `us5.datadoghq.com` |
| EU1 | `datadoghq.eu` |
| AP1 | `ap1.datadoghq.com` |
| US1-FED (GovCloud) | `ddog-gov.com` |

```bash
export DD_SITE=datadoghq.eu
```

---

## Passing credentials

### .env file (local development)

```bash
cp .env.example .env
# Edit .env and fill in your keys
```

```bash
# Docker
docker run --rm --env-file .env -v $(pwd)/policies:/policies:ro ghcr.io/mbaitelman/leash:latest run

# Direct binary
source .env && ./leash run --policy ./policies/
```

**Never commit `.env` to source control.** The provided `.gitignore` excludes it.

### Shell environment

```bash
export DD_API_KEY=your-api-key
export DD_APP_KEY=your-app-key
export DD_SITE=datadoghq.com

./leash run --policy ./policies/
```

### CI/CD — GitHub Actions

Store credentials as [encrypted secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets) and pass them as environment variables:

```yaml
# .github/workflows/leash.yml
name: Leash governance scan

on:
  schedule:
    - cron: "0 6 * * *"   # daily at 06:00 UTC
  workflow_dispatch:

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Leash
        env:
          DD_API_KEY: ${{ secrets.DD_API_KEY }}
          DD_APP_KEY: ${{ secrets.DD_APP_KEY }}
          DD_SITE: ${{ secrets.DD_SITE }}
        run: |
          docker run --rm \
            -e DD_API_KEY -e DD_APP_KEY -e DD_SITE \
            -v ${{ github.workspace }}/policies:/policies:ro \
            ghcr.io/mbaitelman/leash:latest run --policy /policies/ --output-file findings.json

      - name: Upload findings
        uses: actions/upload-artifact@v4
        with:
          name: leash-findings
          path: findings.json
```

### CI/CD — policy validation in PRs

Validate policy files on every pull request without needing Datadog credentials:

```yaml
name: Validate Leash policies

on:
  pull_request:
    paths:
      - "policies/**"

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Validate policies
        run: |
          docker run --rm \
            -v ${{ github.workspace }}/policies:/policies:ro \
            ghcr.io/mbaitelman/leash:latest validate --policy /policies/
```

`leash validate` does not call the Datadog API and requires no credentials.

### Kubernetes — secret mount

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: leash-credentials
type: Opaque
stringData:
  DD_API_KEY: "your-api-key"
  DD_APP_KEY: "your-app-key"
  DD_SITE: "datadoghq.com"
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: leash-scan
spec:
  schedule: "0 6 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: leash
              image: ghcr.io/mbaitelman/leash:latest
              args: ["run", "--policy", "/policies/"]
              envFrom:
                - secretRef:
                    name: leash-credentials
              volumeMounts:
                - name: policies
                  mountPath: /policies
                  readOnly: true
          volumes:
            - name: policies
              configMap:
                name: leash-policies
```

---

## Key rotation

1. Create a new API key and Application key in Datadog (do not delete the old ones yet).
2. Update your secret store / environment with the new values.
3. Run `leash validate` to confirm the new keys are readable.
4. Run `leash run` once and verify findings are produced.
5. Delete the old keys in Datadog.

Leash is stateless — there is no stored session or token cache to flush.

---

## Security considerations

- **Never log credentials.** Leash does not log the values of `DD_API_KEY` or `DD_APP_KEY`.
- **Use scoped App keys.** Grant only the permissions listed above, not `Full Access`.
- **Rotate keys regularly.** Treat API/App keys like passwords — rotate on a schedule and immediately on suspected exposure.
- **Read-only by default.** The default `--dry-run=true` mode requires only read permissions. Enable write permissions on the App key only when you are ready to run with `--dry-run=false`.
- **Separate keys per environment.** Use distinct API/App keys for production governance vs. development testing.
