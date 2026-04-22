# 5-Minute Getting Started

## Prerequisites

- kubectl connected to a cluster (kind, k3s, minikube, or any K8s 1.25+)
- Helm 3.12+

## Step 1: Install (2 min)

```bash
kubectl create namespace hermesmanager

helm install hermesmanager oci://ghcr.io/mackding/charts/hermesmanager \
  --version 1.1.0 \
  --namespace hermesmanager
```

## Step 2: Wait for Readiness (1 min)

```bash
# PostgreSQL (bundled via CloudNativePG, takes 30-60s)
kubectl wait --for=condition=Ready pod \
  -l cnpg.io/cluster=hermesmanager-postgres \
  -n hermesmanager --timeout=5m

# Control plane
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=hermesmanager \
  -n hermesmanager --timeout=5m
```

## Step 3: Port-Forward + Open UI (30s)

```bash
kubectl port-forward svc/hermesmanager 8080:8080 -n hermesmanager &
open http://localhost:8080
```

## Step 4: Get Admin Password (30s)

```bash
kubectl get secret hermesmanager -n hermesmanager \
  -o jsonpath='{.data.admin-password}' | base64 -d && echo
```

## Step 5: Run Your First Task (1 min)

```bash
# Verify readiness
curl -s http://localhost:8080/readyz | jq .

# List skills
curl -s http://localhost:8080/v1/skills | jq '.[].metadata.name'

# Submit a task
curl -X POST http://localhost:8080/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"skill_name":"hello-skill","parameters":{"name":"World"},"runtime":"local"}' \
  | jq .
```

## Next Steps

- [Configuration Reference](../README.md#configuration) -- all Helm values
- [Policy Guide](../deploy/examples/policy.yaml) -- deny/allow rules
- [Agent API](./AGENT_API.md) -- full REST API reference
- [Troubleshooting](./TROUBLESHOOTING.md) -- common errors + fixes
