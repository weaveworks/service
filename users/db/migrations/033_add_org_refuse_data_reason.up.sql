ALTER TABLE organizations ADD COLUMN refuse_data_reason text;
comment on column organizations.refuse_data_reason is 'Override default refuse response (402) with given reason (403)';
