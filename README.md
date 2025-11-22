# HealthCareSystem

Monorepo with:
- backend/: Go API (prescriptions + analytics)
- frontend/: React app (always calls backend API)
- db/: Postgres schema and seed

Assignment 3 (DevOps) — Quick Start
1) Start stack (uses the provided .env):
   docker compose --env-file .env up --build -d

2) Verify services:
   - API health: curl http://localhost:8080/healthz → {"status":"ok"}
   - Web (frontend): http://localhost:5173

3) Stop/Reset:
   - Stop: docker compose down
   - Reset DB (re-run schema/seed on next up): docker compose down -v

Notes
- Compose brings up Postgres, API, and Web:
  - DB initialization: only db/schema.sql and db/seed.sql are mounted into /docker-entrypoint-initdb.d and auto-apply on the first initialization of the data volume (standard Postgres behavior). To re-apply seed, reset the volume with `docker compose down -v`.
  - Do NOT commit real secrets. The committed .env is only for local development defaults; override locally if needed.

Cleanup notes
- Removed legacy db/migrate.sh and db/base_seed.sql as they were not used by docker-compose. The seed in db/seed.sql already covers base data for local development.

API endpoints (RBAC via headers)
- POST /prescriptions
  - Headers: X-Role=physician|patient|admin; X-User-ID=<num>
  - Only physicians may create prescriptions. Patients and admins cannot create. Physicians may only create for linked patients and must match physician_id.
- GET /analytics/top-drugs?from&to&limit=10
  - RFC3339 from/to; limit 1..100. Patients see only their own data; physicians and admins are unrestricted for viewing analytics.
- GET /healthz → {"status":"ok"}

Quick cURL
- Create prescription (physician):
  curl -X POST http://localhost:8080/prescriptions \
    -H 'Content-Type: application/json' -H 'X-Role: physician' -H 'X-User-ID: 1' \
    -d '{"patient_id":1,"physician_id":1,"drug_id":1,"quantity":30,"sig":"1 tab BID"}'
- Top drugs (admin):
  curl 'http://localhost:8080/analytics/top-drugs?from=2025-01-01T00:00:00Z&to=2025-12-31T00:00:00Z' \
    -H 'X-Role: admin' -H 'X-User-ID: 1'

Repo layout
- backend/: Go API and tests
- db/: schema.sql, seed.sql (auto-applied by Postgres on first init)
- frontend/: Vite + React app (talks to backend; no mock mode)

Testing
cd backend && go test ./...
