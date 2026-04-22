# Troubleshooting

Common issues and their solutions. Each entry follows **Symptom / Cause / Fix**.

---

## 1. "connection refused" after helm install

**Symptom**

```
curl: (7) Failed to connect to localhost port 8080: Connection refused
```

Immediately after `helm install`, port-forward fails or curl gets connection refused.

**Cause**

PostgreSQL (via CloudNativePG) takes 30-60 seconds to initialize. The HermesManager pod
cannot start until the database is ready, so the service is not yet listening.

**Fix**

Wait for both pods to become ready before port-forwarding:

```bash
kubectl wait --for=condition=Ready pod \
  -l cnpg.io/cluster=hermesmanager-postgres \
  -n hermesmanager --timeout=5m

kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=hermesmanager \
  -n hermesmanager --timeout=5m
```

---

## 2. "permission denied" running binary

**Symptom**

```
bash: ./hermesmanager: Permission denied
```

**Cause**

The downloaded binary does not have the executable bit set.

**Fix**

```bash
chmod +x ./hermesmanager
./hermesmanager --version
```

---

## 3. "postgres connection failed" with hint

**Symptom**

```
FATAL: postgres connection failed: dial tcp 127.0.0.1:5432: connect: connection refused
```

or authentication errors when running the binary directly.

**Cause**

`DATABASE_URL` is missing, malformed, or points to a Postgres instance that is not running.

**Fix**

Verify the connection string format and that Postgres is reachable:

```bash
# Expected format
export DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=disable"

# Test connectivity
psql "$DATABASE_URL" -c "SELECT 1"
```

If using docker-compose for local development:

```bash
docker compose up -d postgres
# Wait until healthy
docker compose exec postgres pg_isready -U hermesmanager
```

---

## 4. "policy load failed: yaml: ..."

**Symptom**

```
ERR policy load failed error="yaml: line 4: did not find expected key"
```

**Cause**

The policy YAML file has a syntax error -- commonly a tab character, incorrect indentation,
or a missing colon.

**Fix**

Validate your policy file before applying:

```bash
# Install yamllint if needed: pip install yamllint
yamllint deploy/examples/policy.yaml

# Or use Python for a quick syntax check
python3 -c "import yaml, sys; yaml.safe_load(open(sys.argv[1]))" policy.yaml
```

Common pitfalls:
- YAML uses **spaces**, never tabs
- Keys require a colon followed by a space (`action: deny`, not `action:deny`)
- Strings with special characters need quoting

---

## 5. Skills list empty in web UI

**Symptom**

The Skills page in the web UI shows no skills, or the API returns an empty array:

```bash
curl -s http://localhost:8080/v1/skills | jq .
# []
```

**Cause**

Skill definitions are loaded from a ConfigMap. Either:
1. The ConfigMap is not mounted into the pod
2. The skill YAML files have syntax errors
3. The ConfigMap was not created / does not exist in the namespace

**Fix**

Check that the ConfigMap exists and contains valid skill definitions:

```bash
# List ConfigMaps
kubectl get configmap -n hermesmanager

# Inspect skill definitions
kubectl get configmap hermesmanager-skills -n hermesmanager -o yaml

# Validate skill YAML syntax
yamllint deploy/examples/hello-skill.yaml
```

If the ConfigMap is missing, create it from your skill files:

```bash
kubectl create configmap hermesmanager-skills \
  --from-file=hello-skill.yaml=deploy/examples/hello-skill.yaml \
  -n hermesmanager
```

Then restart the pod to pick up the new ConfigMap:

```bash
kubectl rollout restart deployment hermesmanager -n hermesmanager
```

---

## 6. Slack `/hermes` not responding

**Symptom**

The Slack slash command `/hermes` does nothing or returns "dispatch_failed".

**Cause**

Either:
1. Slack integration is disabled in Helm values (`slack.enabled: false`)
2. The Slack bot token or signing secret is incorrect
3. The Slack app's request URL does not point to your HermesManager instance

**Fix**

1. Verify Slack is enabled in your Helm values:

```bash
helm get values hermesmanager -n hermesmanager | grep -A5 slack
```

Ensure `slack.enabled: true` and that `slack.botToken` / `slack.signingSecret` are set.

2. Check the pod logs for Slack-related errors:

```bash
kubectl logs -l app.kubernetes.io/name=hermesmanager -n hermesmanager | grep -i slack
```

3. Verify your Slack app's request URL matches your ingress/service endpoint.

---

## 7. Pod CrashLoopBackOff

**Symptom**

```
kubectl get pods -n hermesmanager
NAME                             READY   STATUS             RESTARTS
hermesmanager-xxxxx-yyyyy        0/1     CrashLoopBackOff   5
```

**Cause**

The container is crashing on startup. The most common reasons are covered in items 1-6
above (database not ready, bad config, invalid policy YAML, missing secrets).

**Fix**

1. Check the pod logs for the fatal error:

```bash
kubectl logs -l app.kubernetes.io/name=hermesmanager -n hermesmanager --previous
```

2. Look for these patterns in the output:

| Log message | See |
|---|---|
| `postgres connection failed` | Issue #3 above |
| `policy load failed` | Issue #4 above |
| `slack` errors | Issue #6 above |
| `bind: address already in use` | Another process on port 8080; change `server.port` in values |
| `secret not found` | Ensure all required secrets exist in the namespace |

3. If the error is not in this list, open an issue with the full log output:
   [github.com/MackDing/hermes-manager/issues](https://github.com/MackDing/hermes-manager/issues)
