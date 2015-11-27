# Migration

How to copy data from one users DB instance to another one.

# Dump old DB

You have to dump from a host in the VPC for the DB.

```
laptop$ grep -i public infrastructure/terraform/dev.tfstate
 ...
public_ip_addr
 ...
laptop$ ssh -i infrastructure/terraform/dev-keypair.pem ubuntu@public_ip_addr

ubuntu$ sudo apt-get install postgresql-client
ubuntu$ export PGPASSWORD="old_user_db_password"
ubuntu$ pg_dump -U postgres -h "old_users_db_host" --dbname=users | tee data.sql

laptop$ scp -i keypair ubuntu@52.23.196.186:data.sql .
```

# Load new DB

You have to load from a host in the VPC for the new DB.

```
laptop$ infra/database hostname dev users_database
new_user_db_host
laptop$ infra/database password dev users_database
new_user_db_password
laptop$ infra/minions dev|head -n1
dev_minion_ip
laptop$ scp -i infra/dev/kube_aws_rsa data.sql ubuntu@dev_minion_ip:
laptop$ ssh -i infra/dev/kube_aws_rsa ubuntu@dev_minion_ip

ubuntu$ sudo apt-get install postgresql-client
ubuntu$ export PGPASSWORD="new_user_db_password"
ubuntu$ psql -U ubuntu -h "new_user_db_host" --dbname=users < data.sql
```

# Confirm

```
laptop$ ssh -i infra/dev/kube_aws_rsa ubuntu@dev_minion_ip

ubuntu$ export PGPASSWORD="new_user_db_password"
ubuntu$ psql -U ubuntu -h "new_user_db_host"
users=> SELECT * FROM users;
```

