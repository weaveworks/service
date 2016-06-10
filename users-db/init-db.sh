#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
  CREATE DATABASE users WITH ENCODING = 'UTF-8';
  CREATE DATABASE users_test WITH ENCODING = 'UTF-8';
EOSQL
