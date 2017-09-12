DROP INDEX organizations_lower_external_id_idx;
CREATE UNIQUE INDEX organizations_external_id ON organizations USING btree(external_id);
