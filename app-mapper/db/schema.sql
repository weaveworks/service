CREATE DATABASE app_mapper WITH ENCODING = 'UTF-8';

\c app_mapper;

CREATE TABLE hosts (
  host  text PRIMARY KEY
);

CREATE TABLE org_host (
  organization_id text,
  host            text REFERENCES hosts(host),
  PRIMARY KEY (organization_id, host)
);
