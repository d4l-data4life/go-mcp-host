# Deployment Examples

This directory contains example deployment configurations for different environments.

## Available Examples

### `local/` - Local Development

For running on local Kubernetes clusters (Docker Desktop, minikube, kind):

- Includes debug mode and verbose logging
- Optional bundled PostgreSQL
- No TLS required
- Permissive CORS settings

**Usage:**
```bash
# Copy and customize
cp local/values.yaml my-local-values.yaml
cp local/secrets.example.yaml local/secrets.yaml

# Edit with your values
vim my-local-values.yaml
vim local/secrets.yaml

# Deploy
make deploy ENVIRONMENT=local VALUES_YAML=my-local-values.yaml
```

### `kubernetes/` - Development/Staging

For non-production Kubernetes environments:

- Moderate logging
- TLS with Let's Encrypt staging
- Resource limits set
- Multiple replicas for testing HA
- Uses external managed database

**Usage:**
```bash
cp kubernetes/values.yaml my-dev-values.yaml
# Edit CHANGE_ME sections
vim my-dev-values.yaml

# Create secrets
kubectl create secret generic go-mcp-host-db-credentials \
  --from-literal=username=mcphost \
  --from-literal=password=<password>

# Deploy
helm upgrade --install go-mcp-host ../helm-chart \
  -f my-dev-values.yaml \
  --namespace dev
```

### `production/` - Production

For production Kubernetes environments:

- Debug mode disabled
- Production TLS certificates
- High availability (3+ replicas)
- Strict resource limits
- Pod disruption budget
- Horizontal pod autoscaling
- Monitoring annotations

**Usage:**
```bash
cp production/values.yaml my-prod-values.yaml
# Carefully review and edit all CHANGE_ME sections
vim my-prod-values.yaml

# Create secrets with secure values
kubectl create secret generic go-mcp-host-db-credentials \
  --from-literal=username=mcphost \
  --from-literal=password=<secure-password> \
  --namespace prod

# Deploy with caution
helm upgrade --install go-mcp-host ../helm-chart \
  -f my-prod-values.yaml \
  --namespace prod \
  --dry-run  # Review first!

# Remove --dry-run when ready
helm upgrade --install go-mcp-host ../helm-chart \
  -f my-prod-values.yaml \
  --namespace prod
```

## Customization Checklist

Before deploying to any environment, ensure you've customized:

- [ ] `APP.HOST` - Your domain name
- [ ] `APP.TLS` - Your TLS secret name (if using HTTPS)
- [ ] `APP.CORS_HOSTS` - Your frontend URLs
- [ ] `APP.REMOTE_KEYS_URL` - Your JWT provider (if using)
- [ ] `imageTag` - Your container registry and image version
- [ ] `DB_*` secrets - Database credentials and connection details
- [ ] MCP server configuration in ConfigMap
- [ ] Resource limits appropriate for your workload

## MCP Server Configuration

All environments require a ConfigMap with MCP server configuration:

```bash
# Create from the example config
kubectl create configmap go-mcp-host-config \
  --from-file=config.yaml=../../config.example.yaml \
  --namespace <your-namespace>

# Or create your own config
cat > config.yaml <<EOF
mcp_servers:
  - name: weather
    type: stdio
    command: npx
    args: ["-y", "@h1deya/mcp-server-weather"]
    enabled: true
EOF

kubectl create configmap go-mcp-host-config \
  --from-file=config.yaml \
  --namespace <your-namespace>
```

## Database Setup

For all environments, ensure your PostgreSQL database is ready:

```bash
# Create database and user
psql -h <db-host> -U postgres <<EOF
CREATE DATABASE mcphost;
CREATE USER mcphost WITH ENCRYPTED PASSWORD '<password>';
GRANT ALL PRIVILEGES ON DATABASE mcphost TO mcphost;
EOF

# Run migrations (from project root)
export DB_HOST=<db-host>
export DB_PORT=5432
export DB_NAME=mcphost
export DB_USER=mcphost
export DB_PASS=<password>
export DB_SSL_MODE=require

go run cmd/api/main.go migrate
```

## Best Practices

1. **Never commit secrets** - Keep `secrets.yaml` out of version control
2. **Use separate namespaces** - Isolate environments (dev, staging, prod)
3. **Version your images** - Use specific tags, not `latest` in production
4. **Test in staging first** - Always deploy to non-prod before production
5. **Monitor your deployment** - Set up logging and metrics
6. **Backup your database** - Regular backups are essential
7. **Use external configs** - Keep sensitive configs in a private repository

## External Configuration Repository (Recommended)

For teams, maintain a separate private repository for deployment configs:

```
my-company-mcp-configs/
├── dev/
│   ├── values.yaml
│   ├── secrets.yaml (encrypted)
│   └── config.yaml
├── staging/
│   ├── values.yaml
│   ├── secrets.yaml (encrypted)
│   └── config.yaml
└── prod/
    ├── values.yaml
    ├── secrets.yaml (encrypted)
    └── config.yaml
```

Deploy using external configs:

```bash
helm upgrade --install go-mcp-host ../go-mcp-host/deploy/helm-chart \
  -f ../my-company-mcp-configs/prod/values.yaml \
  --namespace prod
```

## Troubleshooting

See the main [deploy/README.md](../README.md) for troubleshooting tips.

