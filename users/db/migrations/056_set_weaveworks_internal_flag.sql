update organizations
set feature_flags = array_append(feature_flags, 'weaveworks-internal')
where id in (
    select distinct o.id
    from organizations o
    inner join team_memberships tm on o.team_id = tm.team_id
    where o.deleted_at is null
        and o.feature_flags @> array['no-billing']
        and not o.feature_flags @> array['weaveworks-internal']
        and tm.user_id not in (
            select u.id
            from users u
            where u.email not like '%@weave.works'
                and u.id is null
        )
);
