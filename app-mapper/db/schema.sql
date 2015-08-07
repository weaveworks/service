CREATE DATABASE app_mapper WITH ENCODING = 'UTF-8';

\c app_mapper;

CREATE TABLE org_hostname (
  organization_id text NOT NULL,
  hostname        text NOT NULL,
  PRIMARY KEY (organization_id, hostname)
);
