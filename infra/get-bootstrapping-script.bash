#!/usr/bin/env bash

FILE="get-k8s-io.bash"

wget -q --show-progress -O ${FILE} https://get.k8s.io

# At the end, after downloading everything, the script calls create_cluster.
# We need to make some changes to the downloaded files before doing that.

echo "Making modifications..."

sed -i '' 's/^create_cluster//' ${FILE}

cat <<EOF >>${FILE}
# If the S3 bucket already exists, don't die.
sed -i 's/^\([ \t]+aws s3 mb "s3:\/\/${AWS_S3_BUCKET}" --region ${AWS_S3_REGION}\)$/\1 || true/' kubernetes/cluster/aws/*.sh >> ${FILE}

create_cluster
EOF

