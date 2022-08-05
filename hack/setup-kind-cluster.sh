#!/bin/bash
set -e -o pipefail

# imports utils
source ./hack/utils.sh

# run commands or install
case "${1:-}" in
  "check")
    log "Checking for required commands"
    check_cmd docker kind kubectl
    log "All required commands are installed"
    ;;
  "metallb")
    log "Installing metallb..."
    install_metallb
    log "✅ installed"
    ;;
  *)
    check_cmd docker kind kubectl
    log "Setting up a kind cluster"
    kind create cluster --name "${CLUSTER_NAME:-kind}"
    install_metallb
  ;;
esac
