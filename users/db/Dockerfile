FROM postgres:9.4

ADD init-db.sh /docker-entrypoint-initdb.d/
ADD migrate /migrate
ADD migrations/*.sql /migrations/
