from collections import namedtuple

Org = namedtuple(
    'Org',
    ('external_id', 'internal_id', 'trial_expires_at', 'zuora_account_number', 'gcp_account_id'))