Your First Cluster
===================================

Create a cluster, we need to create a cluster using the ipfs operator CRD.

Create a file with the following information

.. code-block:: yaml

   apiVersion: cluster.ipfs.io/v1alpha1
   kind: IpfsCluster
   metadata:
     name: ipfs-sample-1
   spec:
     ipfsStorage: 2Gi
     clusterStorage: 2Gi
     replicas: 5
     follows: []
     networking:
       circuitRelays: 1


Adjust the storage requirements to meet your needs.

Once you have made the necessary adjustments, apply it to your cluster with kubectl

.. code-block:: bash

  kubectl create namespace mycluster
  kubectl -n mycluster apply -f ipfs.yaml

Verify that the cluster has started by viewing the status of the cluster.

.. code-block:: bash

  kubectl -n mycluster status ipfs-sample-1
