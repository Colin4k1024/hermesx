CREATE TABLE IF NOT EXISTS safety_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    mode TEXT NOT NULL DEFAULT 'log_only',
    input_patterns JSONB NOT NULL DEFAULT '[]',
    output_rules JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id)
);

CREATE INDEX idx_safety_policies_tenant_id ON safety_policies (tenant_id);
