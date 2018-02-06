# Unified notification and alerting

A service and API for all parts of Weave Cloud to deliver any kind of
"event" which should be notified to users, a UI for users to configure
how they would like those notifications delivered, and the back-end of
in-browser notifications.

Docs: [Requirements](https://docs.google.com/document/d/1qnT7CpWHA7lKsgwZ_ZPKmCtom8CqUeSRsBXQv-5IUEo/),
[Design](https://docs.google.com/document/d/11ID8uNkisqtm_tDPmwFL0CQShTjvPKD_nvxttCwhNR8/)

## To run locally: ##

### Configmanager ###

```
docker run --name=postgres -p 5432:5432 -d --env POSTGRES_DB=notifications postgres:9.5
docker run --name=configmanager -p 8080:80 -d quay.io/weaveworks/notification-configmanager -database.uri postgres://postgres@<this host>/notifications?sslmode=disable -database.migrations migrations
```

## To configure / add / remove an event type ##

Please see the relevant section of
[the production playbook](https://github.com/weaveworks/service-conf/tree/master/docs/PLAYBOOK.md#adding-or-removing-event-types)
