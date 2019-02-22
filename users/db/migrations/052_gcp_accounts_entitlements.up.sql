-- Migrate subscription statuses

-- Pending means the subscription is awaiting approval.
-- PENDING -> ENTITLEMENT_ACTIVATION_REQUESTED
UPDATE gcp_accounts SET subscription_status='ENTITLEMENT_ACTIVATION_REQUESTED'
WHERE subscription_status='PENDING';

-- Active is a subscription that is running.
-- ACTIVE -> ENTITLEMENT_ACTIVE
UPDATE gcp_accounts SET subscription_status='ENTITLEMENT_ACTIVE'
WHERE subscription_status='ACTIVE';

-- Complete are subscriptions that are no longer active (i.e., canceled)
-- COMPLETE -> ENTITLEMENT_CANCELLED
UPDATE gcp_accounts SET subscription_status='ENTITLEMENT_CANCELLED'
WHERE subscription_status='COMPLETE';
