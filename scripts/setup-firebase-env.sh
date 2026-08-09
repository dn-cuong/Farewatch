#!/usr/bin/env bash
# Wire Firebase Auth (Google) into .env for local Docker / Vite.
# Free Spark plan is enough: create a web app, enable Google sign-in, paste keys.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT/.env"
EXAMPLE="$ROOT/.env.example"

if [[ ! -f "$ENV_FILE" ]]; then
  cp "$EXAMPLE" "$ENV_FILE"
  echo "Created $ENV_FILE from .env.example"
fi

echo ""
echo "Firebase Auth setup (free Spark plan)"
echo "======================================"
echo "1. Open https://console.firebase.google.com → create/select a project"
echo "2. Project settings → Your apps → Add web app → copy config"
echo "3. Authentication → Sign-in method → enable Google"
echo "4. Authorized domains should include localhost"
echo ""

if [[ ! -t 0 && -z "${FIREBASE_PROJECT_ID:-}" ]]; then
  echo "Non-interactive shell and no FIREBASE_* env exports."
  echo "Re-run from a terminal, or export:"
  echo "  FIREBASE_PROJECT_ID VITE_FIREBASE_API_KEY VITE_FIREBASE_AUTH_DOMAIN"
  echo "  VITE_FIREBASE_PROJECT_ID VITE_FIREBASE_APP_ID"
  exit 1
fi

prompt() {
  local var="$1" label="$2" default="${3:-}"
  local current existing
  existing="$(grep -E "^${var}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- || true)"
  if [[ -n "${!var:-}" ]]; then
    current="${!var}"
  elif [[ -n "$existing" ]]; then
    current="$existing"
  else
    current="$default"
  fi
  if [[ -t 0 ]]; then
    local hint="$current"
    if [[ -n "$hint" && "$var" == *API_KEY* ]]; then
      hint="${hint:0:6}…${hint: -4}"
    fi
    read -r -p "$label [${hint:-empty}]: " input || true
    if [[ -n "${input:-}" ]]; then
      printf -v "$var" '%s' "$input"
    else
      printf -v "$var" '%s' "$current"
    fi
  else
    printf -v "$var" '%s' "$current"
  fi
}

prompt FIREBASE_PROJECT_ID "FIREBASE_PROJECT_ID (same as projectId)"
prompt VITE_FIREBASE_API_KEY "VITE_FIREBASE_API_KEY"
prompt VITE_FIREBASE_AUTH_DOMAIN "VITE_FIREBASE_AUTH_DOMAIN" "${FIREBASE_PROJECT_ID}.firebaseapp.com"
prompt VITE_FIREBASE_PROJECT_ID "VITE_FIREBASE_PROJECT_ID" "$FIREBASE_PROJECT_ID"
prompt VITE_FIREBASE_APP_ID "VITE_FIREBASE_APP_ID"

if [[ -z "$FIREBASE_PROJECT_ID" || -z "$VITE_FIREBASE_API_KEY" || -z "$VITE_FIREBASE_AUTH_DOMAIN" || -z "$VITE_FIREBASE_PROJECT_ID" || -z "$VITE_FIREBASE_APP_ID" ]]; then
  echo "Error: all five Firebase values are required." >&2
  exit 1
fi

if [[ -z "$VITE_FIREBASE_AUTH_DOMAIN" ]]; then
  VITE_FIREBASE_AUTH_DOMAIN="${FIREBASE_PROJECT_ID}.firebaseapp.com"
fi
if [[ -z "$VITE_FIREBASE_PROJECT_ID" ]]; then
  VITE_FIREBASE_PROJECT_ID="$FIREBASE_PROJECT_ID"
fi

upsert() {
  local key="$1" value="$2"
  if grep -qE "^${key}=" "$ENV_FILE"; then
    # portable in-place replace
    local tmp
    tmp="$(mktemp)"
    awk -v k="$key" -v v="$value" 'BEGIN{FS=OFS="="} $1==k{$0=k"="v} {print}' "$ENV_FILE" >"$tmp"
    mv "$tmp" "$ENV_FILE"
  else
    printf '\n%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

upsert FIREBASE_PROJECT_ID "$FIREBASE_PROJECT_ID"
upsert VITE_FIREBASE_API_KEY "$VITE_FIREBASE_API_KEY"
upsert VITE_FIREBASE_AUTH_DOMAIN "$VITE_FIREBASE_AUTH_DOMAIN"
upsert VITE_FIREBASE_PROJECT_ID "$VITE_FIREBASE_PROJECT_ID"
upsert VITE_FIREBASE_APP_ID "$VITE_FIREBASE_APP_ID"

echo ""
echo "Wrote Firebase keys to $ENV_FILE"
echo "Next: make up   # rebuilds web image with VITE_* bake-in"
echo "Then open http://localhost:3000/login - Google button should appear."
