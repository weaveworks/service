CREATE SEQUENCE webhooks_id_seq;
CREATE TABLE IF NOT EXISTS webhooks (
    id                  text PRIMARY KEY NOT NULL DEFAULT nextval('webhooks_id_seq'::regclass),
    organization_id     text NOT NULL REFERENCES organizations(id),
    integration_type    text NOT NULL,
    secret_id           text,
    secret_signing_key  text,

    created_at  timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at  timestamp with time zone
);

CREATE UNIQUE INDEX webhooks_organization_id_type_secret_id ON webhooks (organization_id, integration_type, secret_id) WHERE deleted_at IS NULL;
CREATE INDEX webhooks_secret_id ON webhooks (secret_id);
