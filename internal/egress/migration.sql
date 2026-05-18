CREATE TABLE egress_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    host_pattern TEXT NOT NULL,
    path_prefix TEXT DEFAULT '/',
    action TEXT NOT NULL DEFAULT 'allow',
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_egress_rules_tenant ON egress_rules(tenant_id);
