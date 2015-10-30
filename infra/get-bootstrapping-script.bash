#!/usr/bin/env bash

FILE="get-k8s-io.bash"

wget -q --show-progress -O ${FILE} https://get.k8s.io


echo "Modifying the script..."

# The bootstrapping script should download everything and set up vars.
# Let us invoke create_cluster or kube-down on our own.
sed -i'.bak' 's/^create_cluster//' ${FILE} ; rm -f *.bak

cat <<EOF >>${FILE}
# If the S3 bucket already exists, don't die.
sed -i'.bak' 's/^\(.*aws s3 mb.*\)$/\1 || true/' kubernetes/cluster/aws/*.sh
EOF

