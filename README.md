# FareWatch

## Overview

FareWatch is a flight price tracker. Search a route, watch a fare, and get an email when the price drops.

The React UI talks to a Go GraphQL API. A rate-limited worker pool polls airline adapters and optional search providers, caches recent quotes in Redis, and stores price history in PostgreSQL. Locally everything runs via Docker Compose. Production can sit on AWS (EKS, RDS, ElastiCache, ALB) with a Kubernetes CronJob for scheduled scans.

## Tech Stack

| Group | Skills |
|------|--------|
| Languages | ![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white) ![TypeScript](https://img.shields.io/badge/TypeScript-%23007ACC.svg?style=for-the-badge&logo=typescript&logoColor=white) ![JavaScript](https://img.shields.io/badge/JavaScript-%23323330.svg?style=for-the-badge&logo=javascript&logoColor=%23F7DF1E) ![SQL](https://img.shields.io/badge/SQL-4479A1?style=for-the-badge&logo=postgresql&logoColor=white) ![HTML5](https://img.shields.io/badge/HTML5-%23E34F26.svg?style=for-the-badge&logo=html5&logoColor=white) ![CSS3](https://img.shields.io/badge/CSS3-%231572B6.svg?style=for-the-badge&logo=css3&logoColor=white) |
| Frameworks & Libraries | ![React](https://img.shields.io/badge/React-%2320232a.svg?style=for-the-badge&logo=react&logoColor=%2361DAFB) ![Vite](https://img.shields.io/badge/Vite-646CFF?style=for-the-badge&logo=vite&logoColor=white) ![GraphQL](https://img.shields.io/badge/GraphQL-E10098?style=for-the-badge&logo=graphql&logoColor=white) ![Nginx](https://img.shields.io/badge/Nginx-009639?style=for-the-badge&logo=nginx&logoColor=white) ![NodeJS](https://img.shields.io/badge/Node.js-6DA55F?style=for-the-badge&logo=node.js&logoColor=white) |
| Databases & Cloud | ![PostgreSQL](https://img.shields.io/badge/PostgreSQL-%23316192.svg?style=for-the-badge&logo=postgresql&logoColor=white) ![Redis](https://img.shields.io/badge/Redis-%23DD0031.svg?style=for-the-badge&logo=redis&logoColor=white) ![AWS](https://img.shields.io/badge/AWS-%23FF9900.svg?style=for-the-badge&logo=amazon-aws&logoColor=white) ![Firebase](https://img.shields.io/badge/Firebase-FFCA28?style=for-the-badge&logo=firebase&logoColor=black) |
| Tools & DevOps | ![Docker](https://img.shields.io/badge/Docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white) ![Kubernetes](https://img.shields.io/badge/Kubernetes-%23326ce5.svg?style=for-the-badge&logo=kubernetes&logoColor=white) ![Terraform](https://img.shields.io/badge/Terraform-7B42BC?style=for-the-badge&logo=terraform&logoColor=white) ![Git](https://img.shields.io/badge/Git-%23F05033.svg?style=for-the-badge&logo=git&logoColor=white) ![GitHub](https://img.shields.io/badge/GitHub-%23121011.svg?style=for-the-badge&logo=github&logoColor=white) ![Linux](https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black) |

## System Architecture

![FareWatch System Architecture](docs/architecture.png)

## Run locally

**Requires:** [Docker Desktop](https://www.docker.com/products/docker-desktop/)

1. Copy env and start the stack:

```bash
cp .env.example .env
make up
```

2. Open the app:

| Service | URL |
|---------|-----|
| App | http://localhost:3000 |
| GraphQL | http://localhost:8080/graphql |

3. Useful commands:

```bash
make logs   # follow api + web
make scan   # one scanner pass
make down   # stop stack
```

Optional keys in `.env` (Ignav, Travelpayouts, RapidAPI, Firebase, SMTP) unlock live providers, Google sign-in, and email alerts. Without them the stack still runs with simulator fallbacks.

### Google sign-in (optional)

Set `FIREBASE_PROJECT_ID` and `VITE_FIREBASE_*` in `.env`, enable the Google provider in Firebase Console, then:

```bash
make firebase-env   # or edit .env by hand
make up
```

### Dev without rebuilding images

```bash
docker-compose up -d postgres redis
cd backend && go run ./cmd/api
cd frontend && npm install && npm run dev
```

## Layout

```
backend/cmd/api            GraphQL server
backend/cmd/scanner        one-shot scanner (CronJob entrypoint)
backend/internal/airlines  airline + search providers
backend/internal/worker    rate-limited pool
backend/internal/cache     Redis
backend/internal/store     Postgres
frontend/                  React UI (Nginx in production image)
deploy/k8s/                Kubernetes manifests
deploy/terraform/          AWS (EKS, RDS, ElastiCache, ECR)
```

## AWS deploy (optional)

Uses CLI profile `farewatch`. Tear down when finished to stop billing.

```bash
./deploy/scripts/setup-aws-profile.sh   # once
export AWS_PROFILE=farewatch
./deploy/scripts/up.sh
./deploy/scripts/down.sh
```

See `.env.example` for config. MIT licensed - see [LICENSE](LICENSE).
