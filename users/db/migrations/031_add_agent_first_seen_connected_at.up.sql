ALTER TABLE organizations ADD COLUMN first_seen_flux_connected_at timestamp with time zone;
ALTER TABLE organizations ADD COLUMN first_seen_net_connected_at timestamp with time zone;
ALTER TABLE organizations ADD COLUMN first_seen_prom_connected_at timestamp with time zone;
ALTER TABLE organizations ADD COLUMN first_seen_scope_connected_at timestamp with time zone;
