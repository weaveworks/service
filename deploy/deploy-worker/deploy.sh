#!/bin/bash

set -eux

readonly REPO="$1"
readonly DIR="$2"
readonly PRIVATE_KEY="$3"
readonly KUBECONFIG="$4"
readonly IMAGE="$5"
readonly PERIOD=30s

# Clone the repo
export GIT_SSH_COMMAND="ssh -i \"${PRIVATE_KEY}\" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"
git clone "${REPO}" repo/

# Update the config in the repo
kubeimage "${IMAGE}" "repo/${DIR}"

# Save the list of changed files
readonly CHANGED_FILES="$(git --work-tree=repo --git-dir=repo/.git status --porcelain)"
if git --work-tree=repo --git-dir=repo/.git diff --exit-code HEAD >&2; then
    echo "No changes for ${IMAGE}; exiting.." >&2
    exit 1
fi

# Commit and push the change
git config --global user.email "support@example.com"
git config --global user.name "Weave Deploy"
git --work-tree=repo --git-dir=repo/.git commit -a -m "Deploy ${IMAGE}"
git --work-tree=repo --git-dir=repo/.git push

# Run rolling update for every file changes
deploy() {
    local file="$1"

    local namespace="$(grep "namespace:" "${file}"  | cut -d: -f2)"
    if [ -z "${namespace}" ]; then
        echo "Failed to determine namespace for ${file}, assuming 'default'" >&2
        namespace="default"
    fi

    local name_label="$(grep "labels:" -A 2 "${file}" | grep "name:" | cut -d: -f2)"
    if [ -z "${name_label}" ]; then
        echo "Failed to determine name label for ${file}" >&2
        exit 1
    fi

    local old_rc="$(kubectl "--kubeconfig=repo/${KUBECONFIG}" get rc "--namespace=${namespace}" --selector="name=${name_label}" --output=jsonpath='{.items[0].metadata.name}')"
    if [ -z "${old_rc}" ]; then
        echo "Failed to find old replication controller ${file}" >&2
        exit 1
    fi

    kubectl "--kubeconfig=repo/${KUBECONFIG}" "--namespace=${namespace}" rolling-update "${old_rc}" -f "${file}" "--update-period=${PERIOD}"
}

while read flag file; do
    deploy "repo/${file}"
done < <(echo "${CHANGED_FILES}")
