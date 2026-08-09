#!/usr/bin/env bash
# Configure AWS CLI profile "farewatch" for deploy scripts.
set -euo pipefail

PROFILE="${AWS_PROFILE_NAME:-farewatch}"
REGION="${AWS_REGION:-us-east-1}"

echo "Configure AWS profile: ${PROFILE}"
echo "Create an IAM user with enough access for EKS/ECR/RDS/ElastiCache/VPC, then paste keys."
echo ""

if [[ ! -t 0 && -z "${AWS_ACCESS_KEY_ID:-}" ]]; then
  echo "Export AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or run: aws configure --profile ${PROFILE}"
  exit 1
fi

if [[ -n "${AWS_ACCESS_KEY_ID:-}" && -n "${AWS_SECRET_ACCESS_KEY:-}" ]]; then
  KEY="$AWS_ACCESS_KEY_ID"
  SECRET="$AWS_SECRET_ACCESS_KEY"
else
  read -r -p "AWS Access Key ID: " KEY
  read -r -s -p "AWS Secret Access Key: " SECRET
  echo
fi

aws configure set aws_access_key_id "$KEY" --profile "$PROFILE"
aws configure set aws_secret_access_key "$SECRET" --profile "$PROFILE"
aws configure set region "$REGION" --profile "$PROFILE"
aws configure set output json --profile "$PROFILE"

AWS_PROFILE="$PROFILE" AWS_DEFAULT_REGION="$REGION" aws sts get-caller-identity
echo "OK. export AWS_PROFILE=${PROFILE}"
