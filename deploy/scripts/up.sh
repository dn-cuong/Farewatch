#!/usr/bin/env bash
# Provision AWS (EKS + RDS + ElastiCache + ECR), push images, apply K8s apps.
# Default AWS CLI profile: farewatch
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TF_DIR="$ROOT/deploy/terraform"
K8S_DIR="$ROOT/deploy/k8s"
GEN_DIR="$K8S_DIR/generated"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"
AWS_PROFILE="${AWS_PROFILE:-farewatch}"
AWS_REGION="${AWS_REGION:-us-east-1}"
export AWS_PROFILE AWS_DEFAULT_REGION="$AWS_REGION"

log() { printf '\n==> %s\n' "$*"; }

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required tool: $1" >&2
    exit 1
  }
}

need aws
need terraform
need kubectl
need docker
need curl

if ! aws sts get-caller-identity >/dev/null 2>&1; then
  echo "AWS profile '${AWS_PROFILE}' is not configured or invalid." >&2
  echo "Run: ./deploy/scripts/setup-aws-profile.sh" >&2
  exit 1
fi

ACCOUNT="$(aws sts get-caller-identity --query Account --output text)"
log "Using AWS account ${ACCOUNT} profile=${AWS_PROFILE} region=${AWS_REGION}"

# Optional: install helm if missing (needed for AWS Load Balancer Controller).
if ! command -v helm >/dev/null 2>&1; then
  log "Installing helm..."
  if command -v brew >/dev/null 2>&1; then
    brew install helm
  else
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  fi
fi

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

log "terraform init/apply (this takes 15-25+ minutes)..."
cd "$TF_DIR"
terraform init -upgrade
terraform apply -auto-approve

CLUSTER="$(terraform output -raw cluster_name)"
ECR_API="$(terraform output -raw ecr_api_url)"
ECR_WEB="$(terraform output -raw ecr_web_url)"
DATABASE_URL="$(terraform output -raw rds_database_url)"
REDIS_URL="$(terraform output -raw redis_url)"
ALB_ROLE_ARN="$(terraform output -raw alb_controller_role_arn)"
VPC_ID="$(terraform output -raw vpc_id)"

log "kubeconfig for ${CLUSTER}"
aws eks update-kubeconfig --name "$CLUSTER" --region "$AWS_REGION"

log "Install AWS Load Balancer Controller"
helm repo add eks https://aws.github.io/eks-charts >/dev/null
helm repo update >/dev/null
kubectl create serviceaccount aws-load-balancer-controller -n kube-system --dry-run=client -o yaml | kubectl apply -f -
kubectl annotate serviceaccount aws-load-balancer-controller -n kube-system \
  eks.amazonaws.com/role-arn="$ALB_ROLE_ARN" --overwrite

helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName="$CLUSTER" \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller \
  --set region="$AWS_REGION" \
  --set vpcId="$VPC_ID" \
  --wait --timeout 10m

log "ECR login + build/push images tag=${IMAGE_TAG}"
aws ecr get-login-password --region "$AWS_REGION" \
  | docker login --username AWS --password-stdin "${ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com"

docker build --platform linux/amd64 -t "${ECR_API}:${IMAGE_TAG}" "$ROOT/backend"
# Same-origin GraphQL via ALB path /graphql - no ALB DNS needed at build time.
docker build --platform linux/amd64 \
  --build-arg "VITE_API_URL=/graphql" \
  --build-arg "VITE_FIREBASE_API_KEY=${VITE_FIREBASE_API_KEY:-}" \
  --build-arg "VITE_FIREBASE_AUTH_DOMAIN=${VITE_FIREBASE_AUTH_DOMAIN:-}" \
  --build-arg "VITE_FIREBASE_PROJECT_ID=${VITE_FIREBASE_PROJECT_ID:-}" \
  --build-arg "VITE_FIREBASE_APP_ID=${VITE_FIREBASE_APP_ID:-}" \
  -t "${ECR_WEB}:${IMAGE_TAG}" \
  "$ROOT/frontend"

docker push "${ECR_API}:${IMAGE_TAG}"
docker push "${ECR_WEB}:${IMAGE_TAG}"

API_IMAGE="${ECR_API}:${IMAGE_TAG}"
WEB_IMAGE="${ECR_WEB}:${IMAGE_TAG}"

JWT_SECRET="${JWT_SECRET:-$(openssl rand -hex 32)}"
if [[ "$JWT_SECRET" == "farewatch-dev-secret-change-me" ]]; then
  JWT_SECRET="$(openssl rand -hex 32)"
fi

mkdir -p "$GEN_DIR"
render() {
  local src="$1" dest="$2"
  sed \
    -e "s|__ECR_API_IMAGE__|${API_IMAGE}|g" \
    -e "s|__ECR_WEB_IMAGE__|${WEB_IMAGE}|g" \
    -e "s|__FRONTEND_ORIGIN__|http://localhost|g" \
    -e "s|__DATABASE_URL__|${DATABASE_URL}|g" \
    -e "s|__REDIS_URL__|${REDIS_URL}|g" \
    -e "s|__JWT_SECRET__|${JWT_SECRET}|g" \
    -e "s|__FIREBASE_PROJECT_ID__|${FIREBASE_PROJECT_ID:-}|g" \
    -e "s|__IGNAV_API_KEY__|${IGNAV_API_KEY:-}|g" \
    -e "s|__TRAVELPAYOUTS_TOKEN__|${TRAVELPAYOUTS_TOKEN:-}|g" \
    -e "s|__RAPIDAPI_KEY__|${RAPIDAPI_KEY:-}|g" \
    -e "s|__SMTP_HOST__|${SMTP_HOST:-smtp.gmail.com}|g" \
    -e "s|__SMTP_PORT__|${SMTP_PORT:-587}|g" \
    -e "s|__SMTP_FROM__|${SMTP_FROM:-}|g" \
    -e "s|__SMTP_USER__|${SMTP_USER:-}|g" \
    -e "s|__SMTP_PASS__|${SMTP_PASS:-}|g" \
    "$src" >"$dest"
}

log "Render manifests into ${GEN_DIR} (skip in-cluster redis.yaml)"
render "$K8S_DIR/namespace.yaml" "$GEN_DIR/00-namespace.yaml"
render "$K8S_DIR/configmap.yaml" "$GEN_DIR/10-configmap.yaml"
render "$K8S_DIR/secret.example.yaml" "$GEN_DIR/20-secret.yaml"
render "$K8S_DIR/api-deployment.yaml" "$GEN_DIR/30-api.yaml"
render "$K8S_DIR/web-deployment.yaml" "$GEN_DIR/40-web.yaml"
render "$K8S_DIR/scanner-cronjob.yaml" "$GEN_DIR/50-scanner.yaml"
render "$K8S_DIR/ingress.yaml" "$GEN_DIR/60-ingress.yaml"

kubectl apply -f "$GEN_DIR"
kubectl -n farewatch rollout status deploy/farewatch-api --timeout=5m
kubectl -n farewatch rollout status deploy/farewatch-web --timeout=5m

log "Waiting for ALB hostname..."
ALB_HOST=""
for _ in $(seq 1 60); do
  ALB_HOST="$(kubectl -n farewatch get ingress farewatch -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)"
  if [[ -n "$ALB_HOST" ]]; then
    break
  fi
  sleep 10
done

if [[ -z "$ALB_HOST" ]]; then
  echo "ALB hostname not ready yet. Check: kubectl -n farewatch describe ingress farewatch" >&2
  exit 1
fi

ORIGIN="http://${ALB_HOST}"
log "Patch FRONTEND_ORIGIN=${ORIGIN}"
kubectl -n farewatch create configmap farewatch-config \
  --from-literal=APP_ENV=production \
  --from-literal=PORT=8080 \
  --from-literal=FRONTEND_ORIGIN="$ORIGIN" \
  --from-literal=WORKER_COUNT=8 \
  --from-literal=RATE_LIMIT_PER_SEC=20 \
  --from-literal=CACHE_TTL_SECONDS=900 \
  --from-literal=HTTP_RATE_LIMIT_PER_SEC=5 \
  --from-literal=HTTP_RATE_LIMIT_BURST=15 \
  --from-literal=FARE_RETENTION_DAYS=90 \
  --from-literal=SMTP_HOST="${SMTP_HOST:-smtp.gmail.com}" \
  --from-literal=SMTP_PORT="${SMTP_PORT:-587}" \
  --from-literal=SMTP_FROM="${SMTP_FROM:-}" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n farewatch rollout restart deploy/farewatch-api
kubectl -n farewatch rollout status deploy/farewatch-api --timeout=5m

log "Smoke checks"
curl -sf "${ORIGIN}/healthz" && echo
curl -sf -o /dev/null -w "web=%{http_code}\n" "${ORIGIN}/"

cat <<EOF

Deployed.
  Web:     ${ORIGIN}/
  GraphQL: ${ORIGIN}/graphql
  Health:  ${ORIGIN}/healthz

Add Firebase Auth authorized domain: ${ALB_HOST}

Useful checks:
  kubectl get deploy,svc,ingress,cronjob -n farewatch
  kubectl create job --from=cronjob/farewatch-scanner manual-scan -n farewatch
  kubectl logs job/manual-scan -n farewatch

Tear down:
  ./deploy/scripts/down.sh
EOF
