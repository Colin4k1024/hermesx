CREATE TABLE IF NOT EXISTS secret_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name TEXT NOT NULL,
    pattern TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'high',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_secret_patterns_tenant ON secret_patterns(tenant_id);
