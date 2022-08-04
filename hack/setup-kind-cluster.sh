#!/bin/bash
set -e -o pipefail

# imports utils
source ./hack/utils.sh

# run commands or install
case "${1:-}" in
  "check")
    check_kind
    ;;
  "metallb")
    echo "installing metallb..."
    install_metallb
    echo "✅ installed"
    ;;
  *)
    check_cmd docker kind kubectl
    echo "Setting up a kind cluster"
    kind create cluster --name "${CLUSTER_NAME:-kind}"
    install_metallb
  ;;
esac

# check_kind
# setup_kind_cluster
