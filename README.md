# Quality control API

Local HTTP API for plant quality control. One static Go binary talks to PostgreSQL. It migrates itself on startup, so a new binary is installed by replacing the file and restarting the service.

The record model is the usual floor loop:

1. A quality manager defines a product and an inspection plan.
2. An operator opens a lot.
3. An inspector records measurements and defects, then completes the inspection.
4. A failure opens a nonconformance. A supervisor dispositions it.
5. A corrective action can be opened, verified, and closed.
6. Every write is stored in an audit log. Completed measurements keep the limits that were in force at the time.

There is no web UI in this repository. Any client speaks JSON to the API.

## Decision rules

Limits are inclusive. A variable check with no lower or upper limit only records the number.

An inspection fails when any sample is out of spec, a major or critical defect is recorded, or the rejected quantity is above zero. Minor defects alone make the result conditional and put the lot on hold. A clean inspection accepts the lot. Cancelling an inspection puts the lot back to the previous completed result, or to open when nothing has been completed.

A failed inspection opens a nonconformance unless the complete request sends `"open_ncr": false`.

Plans are editable only while draft. Activating a plan freezes it and obsoletes the previous active plan for that product. To change a released plan, revise it and activate the new revision.

## Roles

A higher role can do everything a lower role can do.

| Role | Can do |
| --- | --- |
| viewer | Read |
| operator | Open lots, record results, attach files |
| inspector | Start, complete, and cancel inspections; open nonconformances |
| supervisor | Sites, stations, lot status, disposition, corrective actions, audit log |
| quality_manager | Products, plans, gauges, suppliers, defect catalog; close corrective actions |
| admin | Users and passwords |

The first account is created from `QC_ADMIN_*` only when the users table is empty. Later restarts do not reset that password.

## Requirements

- PostgreSQL 14 or newer
- Go 1.23 or newer, only on the machine that builds the binary

The shipped binary is static. The Ubuntu server does not need a Go toolchain if you copy a Linux build onto it.

## Run with Docker

From this directory, after changing the three secrets:

```bash
export POSTGRES_PASSWORD='a-real-database-password'
export QC_JWT_SECRET="$(openssl rand -base64 48)"
export QC_ADMIN_PASSWORD='a-real-admin-password'
docker compose up --build
```

The API listens on `http://127.0.0.1:8080`. Sign in:

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"a-real-admin-password"}'
```

## Install on Ubuntu

Build the Linux binary on any machine that has Go:

```bash
make build-linux
```

Copy `bin/quality-control-linux-amd64` (or `linux-arm64`) to the server as `/usr/local/bin/quality-control` and mark it executable.

On the server:

```bash
sudo apt update
sudo apt install -y postgresql
sudo adduser --system --group --home /var/lib/quality-control qc
sudo mkdir -p /var/lib/quality-control/attachments
sudo chown -R qc:qc /var/lib/quality-control
```

Edit `deploy/postgres-init.sql` and replace `replace-me`, then:

```bash
sudo -u postgres psql -f deploy/postgres-init.sql
```

Create `/etc/quality-control.env` mode `600`, owned by root. Start from `.env.example`. Set at least:

```bash
QC_HTTP_ADDR=127.0.0.1:8080
QC_DATABASE_URL=postgres://qc:THE-PASSWORD@127.0.0.1:5432/quality_control?sslmode=disable
QC_JWT_SECRET=output-of-make-secret
QC_ADMIN_PASSWORD=a-long-password
QC_ADMIN_EMAIL=admin@localhost
QC_DATA_DIR=/var/lib/quality-control
```

`make secret` prints a JWT secret. The server refuses to start when `QC_JWT_SECRET` is shorter than 32 characters or when the database URL is missing.

```bash
sudo cp deploy/quality-control.service /etc/systemd/system/quality-control.service
sudo systemctl daemon-reload
sudo systemctl enable --now quality-control
sudo systemctl status quality-control
```

The default bind is localhost. For terminals on the plant network, set `QC_HTTP_ADDR=0.0.0.0:8080` and allow that port only from the LAN. Put TLS in front with Caddy or nginx if the clients are browsers on other machines. When the proxy is on the same host, set `QC_TRUST_PROXY=true` so login rate limits see the client address.

Logs go to stdout as JSON. `journalctl -u quality-control -f` shows them.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| QC_HTTP_ADDR | 127.0.0.1:8080 | Listen address |
| QC_DATABASE_URL | required | PostgreSQL URL |
| QC_JWT_SECRET | required, 32+ characters | Access-token signature |
| QC_ACCESS_TTL | 15m | Access token life |
| QC_REFRESH_TTL | 168h | Refresh token life |
| QC_CORS_ORIGINS | empty | Comma-separated browser origins |
| QC_LOG_LEVEL | info | debug, info, warn, error |
| QC_DATA_DIR | ./data | Attachment files |
| QC_TRUST_PROXY | false | Honor X-Forwarded-For from localhost |
| QC_ADMIN_USERNAME | admin | First user, only when the table is empty |
| QC_ADMIN_PASSWORD | empty | Required for that first user |
| QC_ADMIN_EMAIL | admin@localhost | First user email |
| QC_ADMIN_FULL_NAME | Administrator | First user name |
| QC_ENV_FILE | .env | Optional file read at startup |

Values already set in the environment win over the file. systemd should use `EnvironmentFile` and does not need `QC_ENV_FILE`.

## API

All routes except health, version, and auth live under `/api/v1` and require `Authorization: Bearer <access_token>`.

Lists accept `limit` (1–200, default 50) and `offset`. Responses are `{"data": ..., "page": {"limit","offset","total"}}`. One record is `{"data": ...}`. Errors are `{"error": {"code","message"}}`.

| Method | Path | Who |
| --- | --- | --- |
| POST | /api/v1/auth/login | public |
| POST | /api/v1/auth/refresh | public |
| POST | /api/v1/auth/logout | public, body `refresh_token` |
| GET | /api/v1/me | any user |
| POST | /api/v1/me/password | any user |
| GET | /api/v1/enums | any user |
| GET | /api/v1/dashboard | any user |
| GET | /api/v1/measurements | any user |
| GET | /api/v1/users | admin |
| POST | /api/v1/users | admin |
| GET, PATCH | /api/v1/users/{id} | admin |
| POST | /api/v1/users/{id}/password | admin |
| GET, POST, PATCH | /api/v1/sites | read / supervisor |
| GET, POST, PATCH | /api/v1/stations | read / supervisor |
| GET, POST, PATCH | /api/v1/suppliers | read / quality manager |
| GET, POST, PATCH | /api/v1/gauges | read / quality manager |
| GET, POST, PATCH | /api/v1/products | read / quality manager |
| GET, POST, PATCH | /api/v1/defect-codes | read / quality manager |
| GET, POST | /api/v1/plans | read / quality manager |
| GET, PATCH, DELETE | /api/v1/plans/{id} | read / quality manager |
| POST | /api/v1/plans/{id}/activate | quality manager |
| POST | /api/v1/plans/{id}/obsolete | quality manager |
| POST | /api/v1/plans/{id}/revise | quality manager |
| POST | /api/v1/plans/{id}/characteristics | quality manager |
| PATCH, DELETE | /api/v1/characteristics/{id} | quality manager |
| GET, POST, PATCH | /api/v1/lots | read / operator |
| POST | /api/v1/lots/{id}/status | supervisor |
| GET, POST | /api/v1/inspections | read / inspector |
| GET | /api/v1/inspections/{id} | any user |
| POST | /api/v1/inspections/{id}/results | operator |
| POST | /api/v1/inspections/{id}/defects | inspector |
| POST | /api/v1/inspections/{id}/complete | inspector |
| POST | /api/v1/inspections/{id}/cancel | inspector |
| DELETE | /api/v1/inspection-results/{id} | operator |
| DELETE | /api/v1/inspection-defects/{id} | inspector |
| GET, POST, PATCH | /api/v1/nonconformances | read / inspector |
| POST | /api/v1/nonconformances/{id}/disposition | supervisor |
| POST | /api/v1/nonconformances/{id}/close | supervisor |
| POST | /api/v1/nonconformances/{id}/void | supervisor |
| GET, POST, PATCH | /api/v1/corrective-actions | read / supervisor |
| POST | /api/v1/corrective-actions/{id}/start | supervisor |
| POST | /api/v1/corrective-actions/{id}/verify | supervisor |
| POST | /api/v1/corrective-actions/{id}/reopen | supervisor |
| POST | /api/v1/corrective-actions/{id}/cancel | supervisor |
| POST | /api/v1/corrective-actions/{id}/close | quality manager |
| GET | /api/v1/attachments?entity_type=&entity_id= | any user |
| POST | /api/v1/attachments | operator, multipart fields `entity_type`, `entity_id`, `file` |
| GET, DELETE | /api/v1/attachments/{id} | read / supervisor |
| GET | /api/v1/audit | supervisor |
| GET | /api/v1/search?q= | any user |
| GET | /api/v1/metrics | any user |
| GET | /api/v1/request-audit | supervisor |
| GET | /api/v1/stats/defect-rates | any user |
| GET | /api/v1/stats/ncr-aging | any user |
| GET | /api/v1/stats/capa-aging | any user |
| GET | /api/v1/stats/trends | any user |
| GET | /api/v1/reports/inspections.csv | any user |
| GET | /api/v1/reports/nonconformances.csv | any user |
| GET | /api/v1/reports/corrective-actions.csv | any user |
| GET | /api/v1/reports/inspections.pdf | any user |
| GET | /api/v1/reports/summary.pdf | any user |
| GET, POST, PATCH | /api/v1/checklists | read / quality manager |
| POST | /api/v1/checklists/{id}/items | quality manager |
| PATCH, DELETE | /api/v1/checklist-items/{id} | quality manager |
| GET, POST | /api/v1/inspections/{id}/checklists | read / inspector |
| GET | /api/v1/checklist-runs/{id} | any user |
| POST | /api/v1/checklist-runs/{id}/items/{itemID} | inspector |
| POST | /api/v1/checklist-runs/{id}/complete | inspector |
| GET | /api/v1/webhooks/events | supervisor |
| GET, POST, PATCH | /api/v1/webhooks | supervisor |
| GET | /api/v1/webhook-deliveries | supervisor |
| GET | /api/v1/notifications | supervisor |
| POST | /api/v1/notifications/email | supervisor |
| POST | /api/v1/admin/seed-demo | admin |

`GET /healthz` is process health. `GET /readyz` pings PostgreSQL. `GET /api/v1/version` returns the build version. `GET /metrics` is Prometheus text. `GET /api/v1/openapi.yaml` (and `/api/v1/openapi.json`) is the OpenAPI 3 document for all routes.

Useful filters: `q` on names and codes, `active=true|false`, `status`, `product_id`, `site_id`, `lot_id`, `from`, `to` as RFC3339. Measurements need `product_id`, `characteristic_code`, or `from`, and allow `limit` up to 2000.

Attachments accept JPEG, PNG, WebP, PDF, and plain text, up to 25 MB. Files are stored under `QC_DATA_DIR/attachments` and downloaded only through the API.

A gauge past its calibration date can still be used. The result comes back with `calibration_warning: true`.

## Additive analytics and exports

These routes are new. Existing dashboard, audit, attachment, and CRUD shapes are unchanged.

- **Stats.** `GET /api/v1/stats/defect-rates?group_by=product|site|inspector` returns fail and defect rates (optional `from`, `to`, `product_id`, `site_id`, `inspector_id`). `GET /api/v1/stats/ncr-aging` and `/api/v1/stats/capa-aging` return age buckets plus the oldest open items. `GET /api/v1/stats/trends` returns a daily series of inspections, NCRs, and defects.
- **Search.** `GET /api/v1/search?q=` finds lots, inspections, NCRs, and CAPAs by number or text.
- **Reports.** CSV and PDF downloads under `/api/v1/reports/…` accept the same `from`, `to`, `product_id`, `site_id`, and `status` filters where they apply. PDFs are simple text reports built without external libraries.
- **Request audit.** Middleware records method, path, status, duration, and actor into `request_audit_log` (best-effort, never fails the request). Supervisors read it at `GET /api/v1/request-audit`. Mutation history remains on `GET /api/v1/audit`.
- **Checklists.** Quality managers define reusable checklist templates. Inspectors start a run on an inspection, answer items, and complete the run.
- **Webhooks and email outbox.** Supervisors register HTTPS endpoints for events such as `inspection.completed`, `ncr.created`, `checklist.completed`. Deliveries are stored and signed with `X-QC-Signature` when a secret is set. `POST /api/v1/notifications/email` queues mail for an external sender; this process only stores the outbox row.
- **Metrics.** `GET /metrics` (Prometheus) and `GET /api/v1/metrics` (JSON) expose request counters and uptime.
- **Demo seed.** `POST /api/v1/admin/seed-demo` inserts a coherent plant dataset (site, product, plan, lot, inspection, NCR, CAPA, checklist) without deleting existing rows.

iOS clients: see `docs/ios-new-endpoints.md`.

## Schema changes

Put the next change in `internal/db/migrations/003_something.sql`, rebuild, and restart. Files run once, in name order, inside a transaction. Do not edit a file that has already been applied. Migration `002_qc_extensions.sql` adds request audit, checklists, webhooks, notification outbox, and demo seed tables only.

## Backup

```bash
pg_dump --format=custom --file=quality_control.dump quality_control
```

Restore with `pg_restore`. Copy `QC_DATA_DIR` with the dump; the database stores file names, not the bytes. A daily cron of those two copies is enough for a single-plant server.

## Development

```bash
cp .env.example .env
# fill in QC_DATABASE_URL and QC_JWT_SECRET
make run
make test
```

`QC_TEST_DATABASE_URL` pointing at a database whose name contains `test` runs the full HTTP workflow against PostgreSQL. `make check` runs `go vet` and the unit tests.
