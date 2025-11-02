# Deployment Guide

This directory contains deployment configurations for go-mcp-host.

## Helm Chart Deployment

### Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.x
- PostgreSQL database (managed or self-hosted)
- Ollama instance running and accessible

### Quick Start

1. **Create a values file for your environment:**

```bash
cp examples/local/values.yaml my-deployment-values.yaml
```

2. **Edit the values file with your configuration:**

```yaml
APP:
  HOST: mcp-host.mycompany.com
  CORS_HOSTS: "https://myapp.mycompany.com"
  REMOTE_KEYS_URL: ""  # Optional: Add your JWT provider URL

imageTag: myregistry.io/go-mcp-host:v1.0.0
```

3. **Create database secrets:**

```bash
kubectl create secret generic go-mcp-host-db-credentials \
  --from-literal=username=mcphost \
  --from-literal=password=<your-secure-password>

kubectl create configmap go-mcp-host-db-connection \
  --from-literal=host=postgres.default.svc.cluster.local \
  --from-literal=port=5432 \
  --from-literal=database=mcphost \
  --from-literal=sslmode=disable
```

4. **Create MCP configuration:**

```bash
kubectl create configmap go-mcp-host-config \
  --from-file=config.yaml=../config.example.yaml
```

Edit the config.yaml to add your MCP servers.

5. **Install the Helm chart:**

```bash
helm install go-mcp-host ./helm-chart \
  -f my-deployment-values.yaml \
  --namespace mcp-host \
  --create-namespace
```

### Configuration

#### MCP Servers

Create a `config.yaml` file with your MCP server configurations:

```yaml
mcp_servers:
  - name: weather
    type: stdio
    command: npx
    args:
      - "-y"
      - "@h1deya/mcp-server-weather"
    enabled: true
    description: "Weather information server"
  
  - name: my-api
    type: http
    url: "https://api.mycompany.com/mcp"
    headers:
      Authorization: "Bearer YOUR_TOKEN"
    forwardBearer: false  # Set to true to forward user's bearer token
    enabled: true
    description: "My company's API"
```

Deploy it as a ConfigMap:

```bash
kubectl create configmap go-mcp-host-config --from-file=config.yaml
```

#### Database Setup

Run migrations before first deployment:

```bash
# From the project root
export DB_HOST=your-postgres-host
export DB_PORT=5432
export DB_NAME=mcphost
export DB_USER=mcphost
export DB_PASS=your-password
export DB_SSL_MODE=disable

make run-migrations  # Or use go-migrate directly
```

#### Ollama Configuration

Ensure Ollama is accessible from your cluster:

- **In-cluster:** `http://ollama.default.svc.cluster.local:11434`
- **External:** Use ingress or LoadBalancer service

### Example Deployments

See the `examples/` directory for reference configurations:

- **`examples/local/`** - Local Kubernetes (Docker Desktop, minikube)
- **`examples/kubernetes/`** - Production Kubernetes
- **`examples/production/`** - Production with all features enabled

### Environment-Specific Values

Create separate values files for each environment:

```bash
# Development
helm upgrade --install go-mcp-host ./helm-chart \
  -f values.yaml \
  -f examples/kubernetes/values.yaml \
  --namespace dev

# Production
helm upgrade --install go-mcp-host ./helm-chart \
  -f values.yaml \
  -f examples/production/values.yaml \
  --namespace prod
```

### Upgrading

```bash
helm upgrade go-mcp-host ./helm-chart \
  -f my-deployment-values.yaml \
  --namespace mcp-host
```

### Uninstalling

```bash
helm uninstall go-mcp-host --namespace mcp-host
```

### Troubleshooting

#### Check pod status:
```bash
kubectl get pods -n mcp-host
kubectl describe pod <pod-name> -n mcp-host
kubectl logs <pod-name> -n mcp-host
```

#### Check database connection:
```bash
kubectl exec -it <pod-name> -n mcp-host -- sh
# Inside the pod, try connecting to postgres
```

#### Verify MCP server configuration:
```bash
kubectl get configmap go-mcp-host-config -n mcp-host -o yaml
```

## External Configuration (Recommended for Teams)

For teams, keep deployment configurations in a separate private repository:

```bash
# In your private config repo
my-mcp-configs/
├── dev/
│   ├── values.yaml
│   └── config.yaml
├── staging/
│   ├── values.yaml
│   └── config.yaml
└── prod/
    ├── values.yaml
    └── config.yaml

# Deploy from external configs
helm upgrade --install go-mcp-host ./helm-chart \
  -f ../my-mcp-configs/prod/values.yaml \
  --namespace prod
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/d4l-data4life/go-mcp-host/issues
- Documentation: https://github.com/d4l-data4life/go-mcp-host/docs

