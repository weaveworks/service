CREATE DATABASE app_mapper WITH ENCODING = 'UTF-8';

\c app_mapper;

CREATE TABLE org_hostname (
  organization_id text,
  hostname        text,
  is_ready        boolean NOT NULL,
  PRIMARY KEY (organization_id, hostname)
);

CREATE TABLE probe (
  probe_id        text,
  organization_id text      NOT NULL,
  last_seen       timestamp NOT NULL,
  PRIMARY KEY (probe_id)
);
