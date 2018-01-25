-- This extension is available on Amazon RDS by default and is used for generating random uuids
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS event_types (
	name text PRIMARY KEY,
	display_name text NOT NULL,
	description text NOT NULL,
	default_receiver_types text[] NOT NULL,
	-- Feature flag being set means that the event type is only visible when that flag is enabled.
	-- If feature flag is NULL, event type is always visible.
	feature_flag text
);

CREATE TABLE IF NOT EXISTS receivers (
	receiver_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
	instance_id text NOT NULL,
	receiver_type text NOT NULL,
	address_data json
);
CREATE INDEX IF NOT EXISTS receiver_instance_id_idx ON receivers (instance_id);

-- Note primary key ordering receiver_id first for efficient lookup based on receiver_id
CREATE TABLE IF NOT EXISTS receiver_event_types (
	receiver_id uuid NOT NULL REFERENCES receivers ON DELETE CASCADE,
	event_type text NOT NULL REFERENCES event_types,
	PRIMARY KEY (receiver_id, event_type)
);

CREATE TABLE IF NOT EXISTS events (
	event_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
	event_type text NOT NULL REFERENCES event_types,
	instance_id text NOT NULL,
	timestamp timestamp without time zone NOT NULL,
	messages json NOT NULL
);
-- Force btree index (though this is the default anyway) so it's suitable for range queries
CREATE INDEX IF NOT EXISTS event_instance_id_time_idx
	ON events USING BTREE (instance_id, timestamp);

-- Note: This table is used ONLY for checking if instance defaults have been initialized.
-- It is not intended for tracking whether instances exist, nor does it form a foreign key relation.
CREATE TABLE IF NOT EXISTS instances_initialized (
	instance_id text PRIMARY KEY
);
