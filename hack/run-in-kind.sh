#!/bin/sh

# using helm, install the IPFS Cluster Operator into the current cluster
helm upgrade --install --create-namespace -n ipfs-cluster-system \
		ipfs-cluster ./helm/ipfs-operator