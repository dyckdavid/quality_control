# New backend endpoints for the iOS app

Additive API surface only. Existing routes and JSON shapes are unchanged.

Auth for all of these (except OpenAPI/metrics public routes): `Authorization: Bearer <access_token>`.

## Analytics

| Method | Path | Role | Notes |
| --- | --- | --- | --- |
| GET | `/api/v1/stats/defect-rates` | viewer+ | `group_by=product\|site\|inspector`; optional `from`,`to`,`product_id`,`site_id`,`inspector_id` |
| GET | `/api/v1/stats/ncr-aging` | viewer+ | Open NCR age buckets + items |
| GET | `/api/v1/stats/capa-aging` | viewer+ | Open CAPA age buckets + items |
| GET | `/api/v1/stats/trends` | viewer+ | Daily series; same optional filters as defect-rates |
| GET | `/api/v1/search?q=` | viewer+ | Unified search across lots / inspections / NCRs / CAPAs |

## Reports (file downloads)

| Method | Path | Role |
| --- | --- | --- |
| GET | `/api/v1/reports/inspections.csv` | viewer+ |
| GET | `/api/v1/reports/nonconformances.csv` | viewer+ |
| GET | `/api/v1/reports/corrective-actions.csv` | viewer+ |
| GET | `/api/v1/reports/inspections.pdf` | viewer+ |
| GET | `/api/v1/reports/summary.pdf` | viewer+ |

Optional query: `from`, `to`, `product_id`, `site_id`, `status`, `limit`.

## Checklists

| Method | Path | Role |
| --- | --- | --- |
| GET/POST | `/api/v1/checklists` | viewer+ / quality_manager |
| GET/PATCH | `/api/v1/checklists/{id}` | viewer+ / quality_manager |
| POST | `/api/v1/checklists/{id}/items` | quality_manager |
| PATCH/DELETE | `/api/v1/checklist-items/{id}` | quality_manager |
| GET/POST | `/api/v1/inspections/{id}/checklists` | viewer+ / inspector |
| GET | `/api/v1/checklist-runs/{id}` | viewer+ |
| POST | `/api/v1/checklist-runs/{id}/items/{itemID}` | inspector |
| POST | `/api/v1/checklist-runs/{id}/complete` | inspector |

Item types: `pass_fail`, `yes_no`, `text`, `number`.

## Webhooks / notifications

| Method | Path | Role |
| --- | --- | --- |
| GET | `/api/v1/webhooks/events` | supervisor+ |
| GET/POST | `/api/v1/webhooks` | supervisor+ |
| GET/PATCH | `/api/v1/webhooks/{id}` | supervisor+ |
| GET | `/api/v1/webhook-deliveries` | supervisor+ |
| GET | `/api/v1/notifications` | supervisor+ |
| POST | `/api/v1/notifications/email` | supervisor+ |

## Ops / docs / seed

| Method | Path | Role |
| --- | --- | --- |
| GET | `/metrics` | public (Prometheus) |
| GET | `/api/v1/metrics` | viewer+ (JSON) |
| GET | `/api/v1/openapi.yaml` | public |
| GET | `/api/v1/request-audit` | supervisor+ |
| POST | `/api/v1/admin/seed-demo` | admin |

Full machine-readable catalog: `GET /api/v1/openapi.yaml`.
