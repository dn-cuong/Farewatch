#!/usr/bin/env bash
# Tear down FareWatch AWS demo resources to stop billing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TF_DIR="$ROOT/deploy/terraform"
AWS_PROFILE="${AWS_PROFILE:-farewatch}"
AWS_REGION="${AWS_REGION:-us-east-1}"
export AWS_PROFILE AWS_DEFAULT_REGION="$AWS_REGION"

log() { printf '\n==> %s\n' "$*"; }

if ! aws sts get-caller-identity >/dev/null 2>&1; then
  echo "AWS profile '${AWS_PROFILE}' not usable. Aborting destroy." >&2
  exit 1
fi

ACCOUNT="$(aws sts get-caller-identity --query Account --output text)"
log "Destroying FareWatch demo in account ${ACCOUNT} (profile=${AWS_PROFILE})"

if command -v kubectl >/dev/null 2>&1; then
  if kubectl get ns farewatch >/dev/null 2>&1; then
    log "Delete K8s apps/ingress so ALBs are released first"
    kubectl delete ingress --all -n farewatch --ignore-not-found --wait=true || true
    kubectl delete cronjob,deploy,svc,configmap,secret --all -n farewatch --ignore-not-found || true
    # Give AWS LB controller time to delete the ALB.
    sleep 30
  fi
  helm uninstall aws-load-balancer-controller -n kube-system 2>/dev/null || true
fi

log "terraform destroy"
cd "$TF_DIR"
terraform destroy -auto-approve

log "Done."
echo "Double-check AWS console if you want: EKS, RDS, ElastiCache, NAT, ALB should be gone."
