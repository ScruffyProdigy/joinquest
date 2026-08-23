# JoinQuest Environment Configuration

How runtime configuration works for the JoinQuest frontend and deployments ([platform overview](./vision.md)).

This document explains environment injection for local development and the live joinquest.cc production deployment (plus unused legacy staging/production templates kept for reference).

## Overview

JoinQuest uses a Docker-based approach to inject environment variables into the frontend application at runtime. This allows the same Docker image to be deployed to different environments (local, production) with different configurations.

## How It Works

### 1. Docker Entrypoint Script

The frontend Dockerfile includes a script that runs when the container starts:

```dockerfile
# Create a script to inject environment variables
RUN echo '#!/bin/sh' > /docker-entrypoint.d/30-env-injection.sh && \
    echo 'echo "window.env = {" > /usr/share/nginx/html/env.js' >> /docker-entrypoint.d/30-env-injection.sh && \
    echo 'echo "  REACT_APP_ENV: \"$REACT_APP_ENV\"," >> /usr/share/nginx/html/env.js' >> /docker-entrypoint.d/30-env-injection.sh && \
    echo 'echo "  REACT_APP_API_BASE_URL: \"$REACT_APP_API_BASE_URL\"" >> /usr/share/nginx/html/env.js' >> /docker-entrypoint.d/30-env-injection.sh && \
    echo 'echo "};" >> /usr/share/nginx/html/env.js' >> /docker-entrypoint.d/30-env-injection.sh && \
    chmod +x /docker-entrypoint.d/30-env-injection.sh
```

This script creates an `env.js` file in the nginx document root with the current environment variables.

### 2. Frontend Loading

The frontend loads this configuration in `index.html`:

```html
<script src="/env.js" type="module"></script>
```

The frontend can then access these variables via `window.env`:

```javascript
const apiBaseUrl = window.env.REACT_APP_API_BASE_URL;
const environment = window.env.REACT_APP_ENV;
```

## Environment Configurations

### Local Development

**File**: `k8s/env/local.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lobby-frontend-config
  namespace: playhub
  labels: { env: local }
data:
  REACT_APP_ENV: local
  REACT_APP_API_BASE_URL: "http://localhost:8080"
```

**Deployment**: `./scripts/dev.sh` (Docker Compose) — this is the day-to-day local workflow; see [lobby-maintenance.md](./lobby-maintenance.md#local-stack).

A `./scripts/deploy-local.sh` script also exists for deploying to a local minikube cluster (namespace `playhub`), but it's a legacy path not exercised by the current contributor workflow — prefer `dev.sh`.

- Backend GraphQL: `http://localhost:8080/graphql`
- Frontend: `http://localhost:5173`

### Production (joinquest.cc)

**File**: `k8s/env/joinquest.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lobby-frontend-config
  namespace: joinquest
data:
  REACT_APP_ENV: production
  REACT_APP_API_BASE_URL: "https://joinquest.cc"
```

**Deployment**:

```bash
./scripts/build-and-push.sh --push
./scripts/deploy-joinquest.sh
```

This is the canonical, actively-maintained deploy path — documented in full in [lobby-maintenance.md](./lobby-maintenance.md#deploy-to-joinquestcc-gke). It applies `k8s/base/*` with the namespace rewritten from `playhub` to `joinquest`, applies `k8s/env/joinquest.yaml`, runs the migration job, patches game handoff URLs, and restarts deployments.

- Namespace: `joinquest`
- Public URL: `https://joinquest.cc`
- GraphQL: `https://joinquest.cc/graphql`

### Unused legacy templates (staging.yaml / production.yaml)

`k8s/env/staging.yaml` and `k8s/env/production.yaml`, and their matching scripts `scripts/deploy-staging.sh` / `scripts/deploy-production.sh`, predate the JoinQuest rebrand and the move to the `joinquest` namespace. They target namespaces `playhub-staging` / `playhub-production`, placeholder kubectl contexts (`staging-cluster` / `production-cluster`), and domains (`staging.playhub.com`, `playhub.com`) that don't correspond to any live infrastructure. They still exist in the repo and are technically runnable, but they are **not** part of the current deploy flow, aren't listed in [scripts/README.md](../scripts/README.md)'s contributor script table, and shouldn't be used — they're kept only as historical reference. There is currently no staging environment.

## Environment Variables

### Available Variables

- `REACT_APP_ENV`: Environment identifier (`local`, `staging`, `production`)
- `REACT_APP_API_BASE_URL`: Base URL for the GraphQL API

### Adding New Variables

To add new environment variables:

1. **Update the Dockerfile script** to include the new variable:
   ```dockerfile
   echo 'echo "  REACT_APP_NEW_VAR: \"$REACT_APP_NEW_VAR\"," >> /usr/share/nginx/html/env.js' >> /docker-entrypoint.d/30-env-injection.sh
   ```

2. **Add the variable to all environment ConfigMaps**:
   ```yaml
   data:
     REACT_APP_ENV: local
     REACT_APP_API_BASE_URL: "http://localhost:8081"
     REACT_APP_NEW_VAR: "local-value"
   ```

3. **Update the frontend code** to use the new variable:
   ```javascript
   const newVar = window.env.REACT_APP_NEW_VAR;
   ```

## Benefits

1. **Single Docker Image**: The same image works in all environments
2. **Runtime Configuration**: No need to rebuild for different environments
3. **Kubernetes Native**: Uses ConfigMaps for environment-specific values
4. **Secure**: Sensitive values can be stored in Secrets and referenced in ConfigMaps
5. **Easy Deployment**: Simple scripts for each environment

## Troubleshooting

### Check Environment Configuration

```bash
# View the current ConfigMap (joinquest.cc production)
kubectl get configmap lobby-frontend-config -n joinquest -o yaml

# Check if env.js is being generated correctly
kubectl exec -n joinquest deployment/lobby-frontend -- cat /usr/share/nginx/html/env.js
```

### Verify Frontend Access

```bash
# Check if the frontend is serving env.js
curl https://joinquest.cc/env.js

# Or via port-forward
kubectl port-forward -n joinquest svc/lobby-frontend 8080:80 &
curl http://localhost:8080/env.js
```

### Common Issues

1. **env.js not found**: Check if the Docker entrypoint script is running
2. **Wrong API URL**: Verify the ConfigMap values match your environment
3. **CORS errors**: Ensure the API URL is accessible from the frontend domain
4. **Deployed a new ConfigMap value but production still serves the old one**: `env.js` must be served with `Cache-Control: no-store` (`frontend/nginx.conf`, `location = /env.js`) — otherwise Cloudflare and browsers cache it for up to a year like a hashed bundle asset. See [lobby-maintenance.md](lobby-maintenance.md#envjs-caching-jq-54).
