-- Additive QC extensions: request audit, checklists, webhooks, notification outbox.
-- Does not alter any table created by 001_init.sql.

CREATE TABLE request_audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id uuid REFERENCES users (id) ON DELETE SET NULL,
    method text NOT NULL,
    path text NOT NULL,
    status_code integer NOT NULL,
    duration_ms integer NOT NULL DEFAULT 0,
    ip text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT request_audit_status_chk CHECK (status_code >= 100 AND status_code < 600),
    CONSTRAINT request_audit_duration_chk CHECK (duration_ms >= 0)
);

CREATE INDEX request_audit_created_idx ON request_audit_log (created_at DESC);
CREATE INDEX request_audit_actor_idx ON request_audit_log (actor_id, created_at DESC);
CREATE INDEX request_audit_path_idx ON request_audit_log (path, created_at DESC);

CREATE TABLE checklists (
    id uuid PRIMARY KEY,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    product_id uuid REFERENCES products (id) ON DELETE RESTRICT,
    active boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX checklists_code_idx ON checklists (lower(code));
CREATE INDEX checklists_product_idx ON checklists (product_id) WHERE product_id IS NOT NULL;

CREATE TABLE checklist_items (
    id uuid PRIMARY KEY,
    checklist_id uuid NOT NULL REFERENCES checklists (id) ON DELETE CASCADE,
    sequence integer NOT NULL,
    prompt text NOT NULL,
    item_type text NOT NULL DEFAULT 'pass_fail',
    required boolean NOT NULL DEFAULT true,
    help_text text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT checklist_items_sequence_chk CHECK (sequence > 0 AND sequence <= 10000),
    CONSTRAINT checklist_items_type_chk CHECK (item_type IN ('pass_fail', 'yes_no', 'text', 'number'))
);

CREATE UNIQUE INDEX checklist_items_sequence_idx ON checklist_items (checklist_id, sequence);

CREATE TABLE checklist_runs (
    id uuid PRIMARY KEY,
    checklist_id uuid NOT NULL REFERENCES checklists (id) ON DELETE RESTRICT,
    inspection_id uuid NOT NULL REFERENCES inspections (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'in_progress',
    notes text NOT NULL DEFAULT '',
    started_by uuid REFERENCES users (id),
    completed_by uuid REFERENCES users (id),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT checklist_runs_status_chk CHECK (status IN ('in_progress', 'completed', 'cancelled')),
    UNIQUE (checklist_id, inspection_id)
);

CREATE INDEX checklist_runs_inspection_idx ON checklist_runs (inspection_id);

CREATE TABLE checklist_run_items (
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES checklist_runs (id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES checklist_items (id) ON DELETE RESTRICT,
    sequence integer NOT NULL,
    prompt text NOT NULL,
    item_type text NOT NULL,
    required boolean NOT NULL DEFAULT true,
    result text NOT NULL DEFAULT '',
    numeric_value double precision,
    notes text NOT NULL DEFAULT '',
    answered_by uuid REFERENCES users (id),
    answered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, item_id)
);

CREATE INDEX checklist_run_items_run_idx ON checklist_run_items (run_id, sequence);

CREATE TABLE webhooks (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    url text NOT NULL,
    secret text NOT NULL DEFAULT '',
    events text[] NOT NULL,
    active boolean NOT NULL DEFAULT true,
    description text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhooks_events_chk CHECK (cardinality(events) > 0 AND cardinality(events) <= 32)
);

CREATE INDEX webhooks_active_idx ON webhooks (active) WHERE active;

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY,
    webhook_id uuid NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempt_count integer NOT NULL DEFAULT 0,
    response_code integer,
    response_body text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    delivered_at timestamptz,
    CONSTRAINT webhook_deliveries_status_chk CHECK (status IN ('pending', 'delivered', 'failed', 'skipped')),
    CONSTRAINT webhook_deliveries_attempts_chk CHECK (attempt_count >= 0)
);

CREATE INDEX webhook_deliveries_webhook_idx ON webhook_deliveries (webhook_id, created_at DESC);
CREATE INDEX webhook_deliveries_status_idx ON webhook_deliveries (status, created_at DESC);

CREATE TABLE notification_outbox (
    id uuid PRIMARY KEY,
    channel text NOT NULL,
    event_type text NOT NULL,
    destination text NOT NULL,
    subject text NOT NULL DEFAULT '',
    body text NOT NULL,
    payload jsonb,
    status text NOT NULL DEFAULT 'pending',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    CONSTRAINT notification_outbox_channel_chk CHECK (channel IN ('email', 'webhook')),
    CONSTRAINT notification_outbox_status_chk CHECK (status IN ('pending', 'sent', 'failed', 'skipped'))
);

CREATE INDEX notification_outbox_status_idx ON notification_outbox (status, created_at);

CREATE TABLE demo_seed_runs (
    id uuid PRIMARY KEY,
    seeded_by uuid REFERENCES users (id),
    summary text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER checklists_set_updated_at BEFORE UPDATE ON checklists FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_items_set_updated_at BEFORE UPDATE ON checklist_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_runs_set_updated_at BEFORE UPDATE ON checklist_runs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER checklist_run_items_set_updated_at BEFORE UPDATE ON checklist_run_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER webhooks_set_updated_at BEFORE UPDATE ON webhooks FOR EACH ROW EXECUTE FUNCTION set_updated_at();
