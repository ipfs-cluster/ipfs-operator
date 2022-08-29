Getting Started
===================================

This document explains methods that can be used to install the operator onto an existing kubernetes cluster.

No matter which method you choose, the same operator will be installed.

**With OLM**
::
operator-sdk run bundle quay.io/redhat-et-ipfs/ipfs-operator-bundle:v0.0.1 -n ipfs-operator-system

**With helm**
::
helm install ipfs-operator ./helm/ipfs-operator

**manually**
::
make install



Confirm that the operator is installed.

When the operator is installed, a new namespace will be created. Verify the operator is running in the `ipfs-operator` namespace.

Once the operator is installed, you can proceed with installing your first cluster.
