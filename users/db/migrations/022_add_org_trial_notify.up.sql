ALTER TABLE organizations
ADD COLUMN trial_pending_expiry_notified_at timestamp with time zone,
ADD COLUMN trial_expired_notified_at timestamp with time zone;
