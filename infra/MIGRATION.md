# Migration

How to copy data from one users DB instance to another one.

# Dump old DB

You have to dump from a host in the VPC for the old DB.

```
laptop$ grep users_db_uri infrastructure/terraform/dev.tfstate
old_users_db_host
laptop$ grep users_db_password infrastructure/terraform/dev.tfvars
old_users_db_password
laptop$ grep public_ip infrastructure/terraform/dev.tfstate | head -n1
old_minion_ip
laptop$ ssh -i infrastructure/dev-keypair.pem ubuntu@old_minion_ip

ubuntu$ sudo apt-get install postgresql-client
ubuntu$ export PGPASSWORD="old_users_db_password"
ubuntu$ pg_dump -U postgres -h "old_users_db_host" --dbname=users | tee data.sql

laptop$ scp -i infrastructure/dev-keypair.pem ubuntu@old_minion_ip:data.sql .
```

# Load new DB

You have to load from a host in the VPC for the new DB.

```
laptop$ infra/database hostname dev users_database
new_user_db_host
laptop$ infra/database password dev users_database
new_user_db_password
laptop$ infra/minions dev|head -n1
new_minion_ip
laptop$ scp -i infra/dev/kube_aws_rsa data.sql ubuntu@new_minion_ip:
laptop$ ssh -i infra/dev/kube_aws_rsa ubuntu@new_minion_ip

ubuntu$ sudo apt-get install postgresql-client
ubuntu$ export PGPASSWORD="new_users_db_password"
ubuntu$ psql -U postgres -h "new_users_db_host" --dbname=users < data.sql
```

# Confirm

```
laptop$ ssh -i infra/dev/kube_aws_rsa ubuntu@new_minion_ip

ubuntu$ export PGPASSWORD="new_users_db_password"
ubuntu$ psql -U ubuntu -h "new_users_db_host" --dbname=users
users=> SELECT * FROM users;
```
