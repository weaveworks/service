CREATE SEQUENCE teams_id_seq;
CREATE TABLE IF NOT EXISTS teams (
    id                               text PRIMARY KEY NOT NULL DEFAULT nextval('teams_id_seq'::regclass),
    external_id                      text NOT NULL,
    zuora_account_number             text,
    zuora_account_created_at         timestamp with time zone,
    trial_expires_at                 timestamp with time zone NOT NULL,
    trial_pending_expiry_notified_at timestamp with time zone,
    trial_expired_notified_at        timestamp with time zone,
    created_at                       timestamp with time zone NOT NULL DEFAULT now()
);

-- needed because the function that generates externalIDs checks for duplicates
CREATE UNIQUE INDEX teams_lower_external_id on teams(lower(external_id));
CREATE INDEX teams_created_at on teams(created_at);

ALTER TABLE organizations ADD COLUMN team_id text REFERENCES teams(id);

CREATE SEQUENCE team_memberships_id_seq;
CREATE TABLE IF NOT EXISTS team_memberships (
    id         text PRIMARY KEY NOT NULL DEFAULT nextval('team_memberships_id_seq'),
    user_id    text NOT NULL REFERENCES users(id),
    team_id    text NOT NULL REFERENCES teams(id),
    is_default bool, -- can be either true or NULL, and (user_id,true) is unique
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone,

    UNIQUE(user_id, is_default),
    CONSTRAINT is_default_not_false CHECK(is_default <> false)
);

CREATE UNIQUE INDEX team_memberships_user_id_team_id_idx ON team_memberships (user_id, team_id) WHERE deleted_at IS NULL;
