#! /bin/bash
set -e -o pipefail

kubectl apply -n default -f - <<EOF
---
apiVersion: cluster.ipfs.io/v1alpha1
kind: Ipfs
metadata:
  name: ipfs-sample-1
spec:
  url: example.com
  ipfsStorage: 2Gi
  clusterStorage: 2Gi
  public: false
  replicas: 2
  follows: []
EOF
