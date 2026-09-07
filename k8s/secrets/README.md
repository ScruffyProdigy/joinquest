# Kubernetes secrets (local only — never commit)

Deploy secrets for the JoinQuest platform ([docs/vision.md](../../docs/vision.md)). All `*.yaml` here except `*.example.yaml` are **gitignored**. Copy examples, fill in real values, keep them on your machine (backup via 1Password, etc.).

## First-time setup

```bash
cp k8s/secrets/pg-auth.example.yaml       k8s/secrets/pg-auth.yaml
cp k8s/secrets/pg-dsn.example.yaml        k8s/secrets/pg-dsn.yaml
cp k8s/secrets/jwks-secret.example.yaml   k8s/secrets/jwks-secret.yaml   # or: cd backend && go run ./scripts/jwks.go -namespace joinquest -yamlout ../k8s/secrets/jwks-secret.yaml
cp k8s/secrets/lobby-smtp.example.yaml    k8s/secrets/lobby-smtp.yaml
cp k8s/secrets/lobby-game-service.example.yaml k8s/secrets/lobby-game-service.yaml
cp k8s/secrets/lobby-auth-peppers.example.yaml   k8s/secrets/lobby-auth-peppers.yaml
cp k8s/secrets/lobby-openai.example.yaml      k8s/secrets/lobby-openai.yaml
```

Edit each file:

- **`metadata.namespace`** — must match where you deploy (`joinquest`).
- **`lobby-smtp.yaml`** — paste your Resend API key in `SMTP_PASSWORD`. The backend sends via Resend’s HTTP API by default (same key); set `RESEND_USE_SMTP=true` on the deployment to use SMTP instead.
- **`lobby-auth-peppers.yaml`** — paste two generated peppers (`openssl rand -base64 32` × 2) into `MAGIC_LINK_PEPPER` and `LOBBY_GAME_TOKEN_PEPPER`. Recommended for production.
- **`lobby-game-service.yaml`** — optional legacy global token; prefer **`lobby-auth-peppers.yaml`** for per-game `serviceToken` on provision.
- **`lobby-openai.yaml`** — paste your OpenAI API key in `OPENAI_API_KEY` for the spirit-animal avatar flow (real tarot questions + GPT Image mascots). Without it, the backend uses mock responses.

## Apply to a cluster

```bash
kubectl config use-context YOUR_CONTEXT
kubectl apply -f k8s/secrets/
./scripts/deploy-joinquest.sh    # Lobby only
./scripts/deploy-gke.sh          # Lobby + Word Hunt + RPSLS (GKE)
```

Deploy scripts apply secrets from this folder and wire SMTP into `lobby-backend` when `lobby-smtp.yaml` exists.

## Local dev (no Kubernetes)

`./scripts/dev.sh` reads **`k8s/secrets/lobby-smtp.yaml`** (`SMTP_*`), **`lobby-auth-peppers.yaml`** (`MAGIC_LINK_PEPPER`, `LOBBY_GAME_TOKEN_PEPPER`), and **`lobby-openai.yaml`** (`OPENAI_API_KEY`) automatically — same values as GKE. Override in `.env` if needed.

## Per environment

Use **one set of secret files per machine** and change `namespace:` (and passwords/DSN host) when switching clusters — or maintain separate copies outside the repo (e.g. `~/secure/lobby-secrets-joinquest/`).

Do **not** use the same production DB password or JWT key as demo.
