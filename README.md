# FareWatch

Web app for flight fare tracking. Users search a route, review offers from **30+ global airline pricing adapters** and search aggregators, pick itineraries to watch, and get emailed when those fares drop.

| Piece | Role |
|-------|------|
| React web app | Search routes, pick offers to watch, board + history + alerts |
| GraphQL API (Go) | Auth, multi-provider search, watches, scan trigger |
| 30+ airline adapters | Per-carrier HTTP pricing clients across North America, Europe, Asia-Pacific, the Middle East, and Africa |
| Search APIs | Ignav, Travelpayouts, Sky Scrapper (RapidAPI), and Google Flights probe |
| Scanner CronJob | Rate-limited goroutine pool re-polls watched routes |
| Redis | Short-TTL fare cache (cut redundant upstream calls) |
| PostgreSQL | Users, watches, offer history, alerts |
| SMTP | Drop alerts to an account or email-only watch |

**Targets:** concurrently poll 10+ airline/search providers without stampeding upstreams, normalize + dedupe quotes, measure Redis cache hit rate, email on detected drops. Local stack is Docker Compose; cloud shape is EKS Deployments + CronJob, RDS, ElastiCache.

---

## Problem

Fare pages change constantly. Checking ten airlines by hand does not scale, and naive concurrent scrapers get rate-limited or duplicate the same itinerary across sources.

FareWatch separates **discovery** from **tracking**:

1. User searches origin → destination + date
2. GraphQL concurrently polls airline + search providers, **normalizes** to a common `Offer`, then **dedupes** by flight fingerprint (keep cheapest)
3. User selects which offers to watch and sets a target price
4. A scanner CronJob re-polls only active watches through a **rate-limited worker pool**
5. Recent airline quotes hit Redis when still fresh; every observation lands in Postgres
6. Crossing a watch threshold sends SMTP mail and records delivery latency

This is a systems project (workers, cache, GraphQL, scheduling). It is not a booking OTA and does not issue tickets.

---

### User flow

1. **Search** - Browser sends route criteria to GraphQL (`searchFares`).
2. **Multi-API poll** - API fans out (with bounded concurrency) to 30+ airline adapters + every configured search API.
3. **Normalize / dedupe** - Responses map to one `Offer` shape; identical flights from multiple sources collapse to the cheapest.
4. **Choose** - UI shows the offer list; user picks flights to follow.
5. **Watch** - Save to a dashboard account, or create an email-only watch.
6. **Track** - Scanner CronJob / `runScan` builds `routes × providers` jobs; workers share `RATE_LIMIT_PER_SEC`.
7. **Alert** - On a drop under threshold, SMTP fires; dashboard shows watches, history, alerts.

### Data path

| Hop | What happens |
|-----|----------------|
| Browser → GraphQL | HTTPS `/graphql`, JWT auth |
| GraphQL → providers | Concurrent airline + search API fetches |
| GraphQL → Postgres | Users, watches, immutable fare rows, alerts |
| GraphQL / Scanner → Redis | Hot fare cache with TTL (per airline provider) |
| Scanner → worker pool | Rate-limited goroutines poll every provider for watched routes |
| Scanner → SMTP | Email when a watched fare drops |

### Why Redis + Postgres

| Store | Role |
|-------|------|
| **Redis** | Hot fare cache with TTL - airline adapters skip duplicate upstream calls |
| **PostgreSQL** | System of record for users, selected watches, fare observations, alerts |

### Fare providers

| Kind | Providers |
|------|-----------|
| **Airline adapters** | 32 major carriers including Delta, United, Air France, Lufthansa, Emirates, Qatar, Singapore, Cathay, Qantas, ANA, JAL, Korean Air, Turkish, Air India, Ethiopian, and others. Each tries that carrier's public search surface; failures are explicitly tagged `simulator:*` |
| **Search APIs** | Ignav, free Travelpayouts cached fares, free-tier Sky Scrapper/RapidAPI, Google Flights probe, and legacy Amadeus when configured |

Sources are tagged (`airline:DL`, `search:ignav`, …) so you can see which adapter won after dedupe.

---

## Layout

| Path | Role |
|------|------|
| `backend/cmd/api` | GraphQL HTTP server |
| `backend/cmd/scanner` | One-shot scanner (CronJob entrypoint) |
| `backend/internal/airlines` | 32 global airline adapters + Ignav/Google/Travelpayouts/Sky Scrapper + normalize/dedupe |
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
| `searchFares` | Live route search → multi-airline offer list (public) |
| `createWatch` | Persist a user-selected offer + threshold (auth) |
| `createEmailWatch` | Create an email-only alert without an account |
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
# Add one or more free source keys (Ignav, Travelpayouts, RapidAPI)
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

- `api-deployment.yaml` - GraphQL API
- `web-deployment.yaml` - static React site
- `scanner-cronjob.yaml` - fare scans every 15 minutes
- `redis.yaml` - cache
- `configmap.yaml` / `secret.example.yaml` - config

Point `DATABASE_URL` at RDS PostgreSQL and Redis at ElastiCache (or in-cluster Redis). Push images to ECR, apply CronJob for scheduled polling.

```bash
# Tag with an explicit version (matching the deploy manifests) - never :latest,
# so rollouts are reproducible and rollback actually rolls back.
docker build -t farewatch/api:1.0.0 ./backend
docker build -t farewatch/web:1.0.0 \
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
| `APP_ENV` | `development` (default) or `production` - gates GraphiQL, permissive localhost CORS, and the default-JWT-secret boot check |
| `IGNAV_API_KEY` | Ignav live fare source |
| `TRAVELPAYOUTS_TOKEN` | Free cached fare source |
| `RAPIDAPI_KEY` | Sky Scrapper free-tier structured itinerary source |
| `DATABASE_URL` | Postgres |
| `REDIS_URL` | Redis |
| `JWT_SECRET` | API token signing - the API refuses to boot with the default value when `APP_ENV=production` |
| `SMTP_*` | Outbound email for drop alerts |
| `WORKER_COUNT` / `RATE_LIMIT_PER_SEC` | Scanner concurrency |
| `CACHE_TTL_SECONDS` | Redis fare TTL |
| `HTTP_RATE_LIMIT_PER_SEC` / `HTTP_RATE_LIMIT_BURST` | Per-IP throttle on the public `/graphql` endpoint |
| `FARE_RETENTION_DAYS` | How long fare history rows are kept before the scanner prunes them |

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
