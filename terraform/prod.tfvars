# Port forwarded by connect.sh
host = "tcp://localhost:4501"
users_session_secret = "3919365780a79e0658fde4d03e6945a080031ec077b88975898f698c0d298395"
users_email_uri = "smtp://AKIAJC77KJCIV4HRCM2Q:AqliFmKAFLPagTQkd6FuoZHQDolVU195DorateSEKsTO@email-smtp.us-east-1.amazonaws.com:587"
users_database_uri = "postgres://postgres:bUSiJs6GaP1KuvQMi-75snl8@users-db.crwiyf99ij1c.us-east-1.rds.amazonaws.com/users?sslmode=disable"
appmapper_database_uri = "postgres://postgres:oTcF3mkakTafDcjHmVmLA0fr@app-mapper-db.crwiyf99ij1c.us-east-1.rds.amazonaws.com/app_mapper?sslmode=disable"
appmapper_docker_host = "tcp://swarm-master.weave.local:4567"

# Disable dev containers
dev_containers_count = 0

# Run one frontend per host
frontend_count = 3
appmapper_count = 2
uiserver_count = 2
users_count = 2
