# FareWatch

Web app for flight fare tracking. Users search a route, review live offers from multiple airlines, pick the itineraries they care about, and get emailed when those fares drop under a threshold.

| Piece | Role |
|-------|------|
| React web app | Search routes, pick offers to watch, board + history + alerts |
| GraphQL API (Go) | Auth, live search, watches, scan trigger |
| Ignav | External live fare API (multi-airline itineraries) |
| Scanner CronJob | Re-polls only watched routes on a schedule |
| Redis | Short-TTL fare cache (cut redundant upstream calls) |
| PostgreSQL | Users, watches, offer history, alerts |
| SMTP | Drop alerts to the account email |

**Targets:** concurrent polling without stampeding airline APIs, measurable Redis cache hit rate across scans, email delivery recorded after a detected drop. Local stack is Docker Compose; cloud shape is EKS Deployments + CronJob, RDS, ElastiCache.

---

## Problem

Fare pages change constantly. Checking ten airlines by hand does not scale, and naive concurrent scrapers get rate-limited or duplicate the same request across workers.

FareWatch separates **discovery** from **tracking**:

1. User searches origin → destination + date
2. GraphQL calls the live fare API and returns airline offers for that route
3. User selects which offers to watch and sets a target price
4. A scanner CronJob re-polls only active watches through a rate-limited worker pool
5. Recent quotes hit Redis when still fresh; every observation lands in Postgres
6. Crossing a watch threshold sends SMTP mail and records delivery latency

This is a systems project (workers, cache, GraphQL, scheduling). It is not a booking OTA and does not issue tickets.

---

## Architecture

### User flow

1. **Search** — Browser sends route criteria to GraphQL (`searchFares`).
2. **Live lookup** — API queries Ignav for multi-airline itineraries (price, flight number, schedule, aircraft, stops).
3. **Choose** — UI shows the offer list; user picks one or more flights/airlines to follow.
4. **Watch** — `createWatch` stores the selection (user-owned) in Postgres with a threshold.
5. **Track** — Scanner CronJob / `runScan` re-polls watched routes only; workers share a token bucket (`RATE_LIMIT_PER_SEC`).
6. **Alert** — On a drop under threshold, SMTP fires; dashboard shows `myWatches`, fare history, and `myAlerts`.

### Data path

| Hop | What happens |
|-----|----------------|
| Browser → GraphQL | HTTPS `/graphql`, JWT auth |
| GraphQL → Ignav | Live search for the requested route |
| GraphQL → Postgres | Users, watches, immutable fare rows, alerts |
| GraphQL / Scanner → Redis | Hot fare cache with TTL |
| Scanner → Ignav | Scheduled re-poll of watched routes |
| Scanner → SMTP | Email when a watched fare drops |

### Why Redis + Postgres

| Store | Role |
|-------|------|
| **Redis** | Hot fare cache with TTL — search and scanner skip duplicate upstream calls |
| **PostgreSQL** | System of record for users, selected watches, fare observations, alerts |

Replicas stay consistent because caching and history depend on shared stores, not process-local memory.

### Live fares

Amadeus Self-Service shut down in July 2026. FareWatch uses **Ignav** as the external fare source (`source: "ignav"`). Set `IGNAV_API_KEY` in `.env`. Without a key, offline airline simulator adapters are available for local development only.

---

## Layout

| Path | Role |
|------|------|
| `backend/cmd/api` | GraphQL HTTP server |
| `backend/cmd/scanner` | One-shot scanner (CronJob entrypoint) |
| `backend/internal/airlines` | Ignav client + airline provider adapters |
| `backend/internal/worker` | Rate-limited goroutine pool |
| `backend/internal/cache` | Redis fare cache |
| `backend/internal/store` | Postgres migrations + queries |
| `backend/internal/scanner` | Scan orchestration + alerts |
| `backend/internal/graph` | GraphQL schema |
| `backend/internal/auth` | JWT (+ optional Firebase token verify) |
| `frontend/` | React + TypeScript (Vite) UI |
| `deploy/k8s/` | Namespace, API/web Deployments, Redis, scanner CronJob |
| `docker-compose.yml` | Local full stack |

---

## GraphQL surface

| Op | Purpose |
|----|---------|
| `register` / `login` | Email+password → JWT |
| `loginWithFirebase` | Optional Google ID token → JWT |
| `searchFares` | Live route search → multi-airline offer list |
| `createWatch` | Persist a user-selected offer + threshold |
| `myWatches` | Dashboard rows + `latestFare` |
| `fares(routeId)` | Price history for charts |
| `runScan` | On-demand re-poll of watched routes |
| `myAlerts` | Recent email alert receipts |
| `removeWatch` | Stop tracking a selection |

---

## Local

Needs Docker Desktop. Copy env and start:

```bash
cp .env.example .env
# set IGNAV_API_KEY=... from https://ignav.com
make up
```

| Service | URL |
|---------|-----|
| Website | http://localhost:3000 |
| GraphQL / GraphiQL | http://localhost:8080/graphql |

```bash
make scan    # run scanner once
make logs    # follow api + web
make down    # stop stack
```

### Dev without rebuilding images

```bash
docker-compose up -d postgres redis
cd backend && go run ./cmd/api
cd frontend && npm install && npm run dev
```

---

## Kubernetes / EKS

Manifests in `deploy/k8s/`:

- `api-deployment.yaml` — GraphQL API
- `web-deployment.yaml` — static React site
- `scanner-cronjob.yaml` — fare scans every 15 minutes
- `redis.yaml` — cache
- `configmap.yaml` / `secret.example.yaml` — config

Point `DATABASE_URL` at RDS PostgreSQL and Redis at ElastiCache (or in-cluster Redis). Push images to ECR, apply CronJob for scheduled polling.

```bash
docker build -t farewatch/api:latest ./backend
docker build -t farewatch/web:latest \
  --build-arg VITE_API_URL=https://api.example.com/graphql \
  ./frontend

kubectl apply -f deploy/k8s/namespace.yaml
cp deploy/k8s/secret.example.yaml deploy/k8s/secret.yaml
# edit secret.yaml
kubectl apply -f deploy/k8s/
```

---

## Configuration

| Variable | Purpose |
|----------|---------|
| `IGNAV_API_KEY` | Live fare API (required for market data) |
| `DATABASE_URL` | Postgres |
| `REDIS_URL` | Redis |
| `JWT_SECRET` | API token signing |
| `SMTP_*` | Outbound email for drop alerts |
| `WORKER_COUNT` / `RATE_LIMIT_PER_SEC` | Scanner concurrency |
| `CACHE_TTL_SECONDS` | Redis fare TTL |

See `.env.example` for the full list.

---

## License

Copyright (c) 2026 FareWatch contributors

FareWatch is open source software licensed under the [MIT License](LICENSE).

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

`SPDX-License-Identifier: MIT`
