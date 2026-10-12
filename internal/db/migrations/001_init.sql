-- Quality control schema for PostgreSQL 14 or newer.
-- Applied once, in a transaction, by the server on startup.

CREATE TABLE users (
    id uuid PRIMARY KEY,
    username text NOT NULL,
    email text NOT NULL,
    password_hash text NOT NULL,
    full_name text NOT NULL,
    role text NOT NULL,
    active boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_role_chk CHECK (role IN ('admin', 'quality_manager', 'supervisor', 'inspector', 'operator', 'viewer'))
);

CREATE UNIQUE INDEX users_username_idx ON users (lower(username));
CREATE UNIQUE INDEX users_email_idx ON users (lower(email));

CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

CREATE TABLE sites (
    id uuid PRIMARY KEY,
    code text NOT NULL,
    name text NOT NULL,
    address text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX sites_code_idx ON sites (lower(code));

CREATE TABLE stations (
    id uuid PRIMARY KEY,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE RESTRICT,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX stations_site_code_idx ON stations (site_id, lower(code));

CREATE TABLE suppliers (
    id uuid PRIMARY KEY,
    code text NOT NULL,
    name text NOT NULL,
    contact_name text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    phone text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX suppliers_code_idx ON suppliers (lower(code));

CREATE TABLE gauges (
    id uuid PRIMARY KEY,
    code text NOT NULL,
    name text NOT NULL,
    serial_number text NOT NULL DEFAULT '',
    calibration_due date,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX gauges_code_idx ON gauges (lower(code));

CREATE TABLE products (
    id uuid PRIMARY KEY,
    part_number text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    unit text NOT NULL DEFAULT 'ea',
    active boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX products_part_number_idx ON products (lower(part_number));

CREATE TABLE inspection_plans (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    revision text NOT NULL,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'draft',
    sampling_type text NOT NULL DEFAULT 'hundred_percent',
    aql text NOT NULL DEFAULT '',
    sample_size integer,
    notes text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id),
    approved_by uuid REFERENCES users (id),
    approved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT inspection_plans_status_chk CHECK (status IN ('draft', 'active', 'obsolete')),
    CONSTRAINT inspection_plans_sampling_chk CHECK (sampling_type IN ('hundred_percent', 'fixed', 'aql')),
    CONSTRAINT inspection_plans_sample_size_chk CHECK (sample_size IS NULL OR (sample_size > 0 AND sample_size <= 500))
);

CREATE UNIQUE INDEX inspection_plans_revision_idx ON inspection_plans (product_id, lower(revision));
CREATE UNIQUE INDEX inspection_plans_one_active_idx ON inspection_plans (product_id) WHERE status = 'active';
CREATE INDEX inspection_plans_product_idx ON inspection_plans (product_id, created_at DESC);

CREATE TABLE characteristics (
    id uuid PRIMARY KEY,
    plan_id uuid NOT NULL REFERENCES inspection_plans (id) ON DELETE CASCADE,
    sequence integer NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    characteristic_type text NOT NULL,
    method text NOT NULL DEFAULT '',
    nominal double precision,
    lower_limit double precision,
    upper_limit double precision,
    unit text NOT NULL DEFAULT '',
    critical boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT characteristics_type_chk CHECK (characteristic_type IN ('variable', 'attribute')),
    CONSTRAINT characteristics_sequence_chk CHECK (sequence > 0 AND sequence <= 10000),
    CONSTRAINT characteristics_limits_chk CHECK (lower_limit IS NULL OR upper_limit IS NULL OR lower_limit <= upper_limit)
);

CREATE UNIQUE INDEX characteristics_code_idx ON characteristics (plan_id, lower(code));
CREATE UNIQUE INDEX characteristics_sequence_idx ON characteristics (plan_id, sequence);

CREATE TABLE defect_codes (
    id uuid PRIMARY KEY,
    code text NOT NULL,
    name text NOT NULL,
    severity text NOT NULL,
    category text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT defect_codes_severity_chk CHECK (severity IN ('minor', 'major', 'critical'))
);

CREATE UNIQUE INDEX defect_codes_code_idx ON defect_codes (lower(code));

CREATE TABLE lots (
    id uuid PRIMARY KEY,
    lot_number text NOT NULL,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    site_id uuid NOT NULL REFERENCES sites (id) ON DELETE RESTRICT,
    supplier_id uuid REFERENCES suppliers (id) ON DELETE RESTRICT,
    quantity double precision NOT NULL,
    unit text NOT NULL DEFAULT 'ea',
    source text NOT NULL,
    supplier_lot text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open',
    received_at timestamptz NOT NULL DEFAULT now(),
    notes text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT lots_quantity_chk CHECK (quantity > 0),
    CONSTRAINT lots_source_chk CHECK (source IN ('incoming', 'in_process', 'final', 'customer_return')),
    CONSTRAINT lots_status_chk CHECK (status IN ('open', 'in_inspection', 'accepted', 'rejected', 'on_hold', 'closed'))
);

CREATE UNIQUE INDEX lots_number_idx ON lots (lower(lot_number));
CREATE INDEX lots_status_idx ON lots (status, created_at DESC);
CREATE INDEX lots_product_idx ON lots (product_id, created_at DESC);
CREATE INDEX lots_site_idx ON lots (site_id, created_at DESC);

CREATE TABLE inspections (
    id uuid PRIMARY KEY,
    inspection_number text NOT NULL,
    lot_id uuid NOT NULL REFERENCES lots (id) ON DELETE RESTRICT,
    plan_id uuid NOT NULL REFERENCES inspection_plans (id) ON DELETE RESTRICT,
    station_id uuid REFERENCES stations (id) ON DELETE RESTRICT,
    inspector_id uuid NOT NULL REFERENCES users (id),
    status text NOT NULL DEFAULT 'in_progress',
    result text NOT NULL DEFAULT 'pending',
    sample_size integer NOT NULL,
    quantity_inspected double precision NOT NULL DEFAULT 0,
    quantity_accepted double precision NOT NULL DEFAULT 0,
    quantity_rejected double precision NOT NULL DEFAULT 0,
    notes text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT inspections_status_chk CHECK (status IN ('in_progress', 'completed', 'cancelled')),
    CONSTRAINT inspections_result_chk CHECK (result IN ('pending', 'pass', 'fail', 'conditional')),
    CONSTRAINT inspections_sample_size_chk CHECK (sample_size > 0 AND sample_size <= 500),
    CONSTRAINT inspections_qty_chk CHECK (quantity_inspected >= 0 AND quantity_accepted >= 0 AND quantity_rejected >= 0)
);

CREATE UNIQUE INDEX inspections_number_idx ON inspections (inspection_number);
CREATE INDEX inspections_lot_idx ON inspections (lot_id, created_at DESC);
CREATE INDEX inspections_status_idx ON inspections (status, created_at DESC);
CREATE INDEX inspections_inspector_idx ON inspections (inspector_id, created_at DESC);

CREATE TABLE inspection_results (
    id uuid PRIMARY KEY,
    inspection_id uuid NOT NULL REFERENCES inspections (id) ON DELETE CASCADE,
    characteristic_id uuid NOT NULL REFERENCES characteristics (id) ON DELETE RESTRICT,
    sample_index integer NOT NULL,
    measured_value double precision,
    attribute_result text,
    in_spec boolean NOT NULL,
    calibration_warning boolean NOT NULL DEFAULT false,
    is_critical boolean NOT NULL DEFAULT false,
    nominal double precision,
    lower_limit double precision,
    upper_limit double precision,
    unit text NOT NULL DEFAULT '',
    characteristic_code text NOT NULL,
    characteristic_name text NOT NULL,
    characteristic_type text NOT NULL,
    gauge_id uuid REFERENCES gauges (id) ON DELETE RESTRICT,
    notes text NOT NULL DEFAULT '',
    recorded_by uuid REFERENCES users (id),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT inspection_results_sample_chk CHECK (sample_index > 0 AND sample_index <= 500),
    CONSTRAINT inspection_results_attribute_chk CHECK (attribute_result IS NULL OR attribute_result IN ('pass', 'fail')),
    CONSTRAINT inspection_results_type_chk CHECK (characteristic_type IN ('variable', 'attribute')),
    UNIQUE (inspection_id, characteristic_id, sample_index)
);

CREATE INDEX inspection_results_inspection_idx ON inspection_results (inspection_id, sample_index);

CREATE TABLE inspection_defects (
    id uuid PRIMARY KEY,
    inspection_id uuid NOT NULL REFERENCES inspections (id) ON DELETE CASCADE,
    defect_code_id uuid NOT NULL REFERENCES defect_codes (id) ON DELETE RESTRICT,
    code text NOT NULL,
    name text NOT NULL,
    severity text NOT NULL,
    quantity integer NOT NULL,
    notes text NOT NULL DEFAULT '',
    recorded_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT inspection_defects_qty_chk CHECK (quantity > 0),
    CONSTRAINT inspection_defects_severity_chk CHECK (severity IN ('minor', 'major', 'critical'))
);

CREATE INDEX inspection_defects_inspection_idx ON inspection_defects (inspection_id);

CREATE TABLE nonconformances (
    id uuid PRIMARY KEY,
    ncr_number text NOT NULL,
    inspection_id uuid REFERENCES inspections (id) ON DELETE RESTRICT,
    lot_id uuid REFERENCES lots (id) ON DELETE RESTRICT,
    product_id uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    title text NOT NULL,
    description text NOT NULL,
    severity text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    disposition text NOT NULL DEFAULT 'pending',
    quantity_affected double precision NOT NULL DEFAULT 0,
    assigned_to uuid REFERENCES users (id),
    opened_by uuid REFERENCES users (id),
    disposed_by uuid REFERENCES users (id),
    closed_by uuid REFERENCES users (id),
    disposition_notes text NOT NULL DEFAULT '',
    opened_at timestamptz NOT NULL DEFAULT now(),
    disposed_at timestamptz,
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ncr_severity_chk CHECK (severity IN ('minor', 'major', 'critical')),
    CONSTRAINT ncr_status_chk CHECK (status IN ('open', 'under_review', 'dispositioned', 'closed', 'void')),
    CONSTRAINT ncr_disposition_chk CHECK (disposition IN ('pending', 'use_as_is', 'rework', 'repair', 'scrap', 'return_to_supplier', 'sort')),
    CONSTRAINT ncr_qty_chk CHECK (quantity_affected >= 0)
);

CREATE UNIQUE INDEX nonconformances_number_idx ON nonconformances (ncr_number);
CREATE INDEX nonconformances_status_idx ON nonconformances (status, opened_at DESC);
CREATE INDEX nonconformances_product_idx ON nonconformances (product_id, opened_at DESC);

CREATE TABLE corrective_actions (
    id uuid PRIMARY KEY,
    ca_number text NOT NULL,
    nonconformance_id uuid REFERENCES nonconformances (id) ON DELETE RESTRICT,
    title text NOT NULL,
    root_cause text NOT NULL DEFAULT '',
    action_plan text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open',
    owner_id uuid REFERENCES users (id),
    due_date date,
    verification_notes text NOT NULL DEFAULT '',
    effectiveness_notes text NOT NULL DEFAULT '',
    opened_by uuid REFERENCES users (id),
    verified_by uuid REFERENCES users (id),
    closed_by uuid REFERENCES users (id),
    opened_at timestamptz NOT NULL DEFAULT now(),
    verified_at timestamptz,
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT capa_status_chk CHECK (status IN ('open', 'in_progress', 'verification', 'closed', 'cancelled'))
);

CREATE UNIQUE INDEX corrective_actions_number_idx ON corrective_actions (ca_number);
CREATE INDEX corrective_actions_status_idx ON corrective_actions (status, due_date);

CREATE TABLE attachments (
    id uuid PRIMARY KEY,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    file_name text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL,
    sha256 text NOT NULL,
    storage_name text NOT NULL UNIQUE,
    uploaded_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT attachments_entity_chk CHECK (entity_type IN ('lot', 'inspection', 'nonconformance', 'corrective_action')),
    CONSTRAINT attachments_size_chk CHECK (size_bytes > 0)
);

CREATE INDEX attachments_entity_idx ON attachments (entity_type, entity_id);

CREATE TABLE audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id uuid REFERENCES users (id),
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid,
    summary text NOT NULL DEFAULT '',
    before_data jsonb,
    after_data jsonb,
    ip text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

CREATE TABLE document_counters (
    doc_type text PRIMARY KEY,
    year integer NOT NULL,
    last_value integer NOT NULL,
    CONSTRAINT document_counters_value_chk CHECK (last_value >= 0)
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER sites_set_updated_at BEFORE UPDATE ON sites FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER stations_set_updated_at BEFORE UPDATE ON stations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER suppliers_set_updated_at BEFORE UPDATE ON suppliers FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER gauges_set_updated_at BEFORE UPDATE ON gauges FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER products_set_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER inspection_plans_set_updated_at BEFORE UPDATE ON inspection_plans FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER characteristics_set_updated_at BEFORE UPDATE ON characteristics FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER defect_codes_set_updated_at BEFORE UPDATE ON defect_codes FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER lots_set_updated_at BEFORE UPDATE ON lots FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER inspections_set_updated_at BEFORE UPDATE ON inspections FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER inspection_results_set_updated_at BEFORE UPDATE ON inspection_results FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER nonconformances_set_updated_at BEFORE UPDATE ON nonconformances FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER corrective_actions_set_updated_at BEFORE UPDATE ON corrective_actions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO defect_codes (id, code, name, severity, category) VALUES
    (gen_random_uuid(), 'WELD-MISS', 'Missed or incomplete weld', 'major', 'weld'),
    (gen_random_uuid(), 'WELD-CRACK', 'Crack in a weld or in the steel', 'critical', 'weld'),
    (gen_random_uuid(), 'EDGE-BURR', 'Sharp edge or burr', 'major', 'fit'),
    (gen_random_uuid(), 'LATCH-OP', 'Latch, hinge, or squeeze does not travel', 'major', 'function'),
    (gen_random_uuid(), 'DIM-OUT', 'Dimension out of print', 'major', 'dimension'),
    (gen_random_uuid(), 'FIT-GAP', 'Fit-up out of print', 'major', 'fit'),
    (gen_random_uuid(), 'PAINT-THIN', 'Thin or missed finish', 'minor', 'finish'),
    (gen_random_uuid(), 'PAINT-RUN', 'Paint run or sag', 'minor', 'finish'),
    (gen_random_uuid(), 'HDW-MISS', 'Missing pin, bolt, or chain', 'minor', 'hardware'),
    (gen_random_uuid(), 'GLASS-CRACK', 'Cracked glass', 'critical', 'hearth'),
    (gen_random_uuid(), 'GASKET-GAP', 'Gasket gap', 'major', 'hearth'),
    (gen_random_uuid(), 'PLATE-MISS', 'Rating plate missing or unreadable', 'critical', 'hearth');
