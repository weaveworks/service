#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
  CREATE DATABASE users WITH ENCODING = 'UTF-8';
  CREATE DATABASE users_test WITH ENCODING = 'UTF-8';
EOSQL

/migrate -url postgres://"$POSTGRES_USER"@localhost:5432/users?sslmode=disable      -path /migrations up
/migrate -url postgres://"$POSTGRES_USER"@localhost:5432/users_test?sslmode=disable -path /migrations up
