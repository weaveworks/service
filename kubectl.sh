script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
parent_script="$(basename "${0}")"
env="${parent_script/kubectl_/}"

# TODO: check if kubectl is present and what version it is

exec kubectl --kubeconfig="${script_dir}/infra/${env}/kubeconfig" "$@"
