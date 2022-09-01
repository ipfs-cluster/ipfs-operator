Getting Started
===================================

This document explains methods that can be used to install the operator onto an existing kubernetes cluster.

No matter which method you choose, the same operator will be installed.

**With helm**
::
helm install ipfs-operator ./helm/ipfs-operator

**manually**
::
make install



Confirm that the operator is installed.

When the operator is installed, a new namespace will be created. Verify the operator is running in the `ipfs-operator` namespace.

Once the operator is installed, you can proceed with installing your first cluster.
