Your First Cluster
===================================

Create a cluster, we need to create a cluster using the ipfs operator CRD.

Create a file with the following information

.. code-block:: yaml

   apiVersion: cluster.ipfs.io/v1alpha1
   kind: Ipfs
   metadata:
     name: ipfs-sample-1
   spec:
     url: apps.example.com 
     ipfsStorage: 2Gi
     clusterStorage: 2Gi
     replicas: 5
     public: true


Adjust the storage requirements to meet your needs.

Once you have made the necessary adjustments, apply it to your cluster with kubectl

.. code-block:: bash

  kubectl create namespace my_cluster
  kubectl -n my_cluster apply -f ipfs.yaml

Verify that the cluster has started by viewing the status of the cluster.

.. code-block:: bash

  kubectl -n my_namespace status ipfs-sample-1
