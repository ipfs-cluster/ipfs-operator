# IPFS Operator
This operator is still heavily in progress.

# Running
This operator can be deployed either with or without OLM installed.

## With OLM
```bash
operator-sdk run bundle quay.io/redhat-et-ipfs/ipfs-operator-bundle:v0.0.1 -n ipfs-operator-system
```

## Without OLM
```bash
make deploy
```

# Deploying an IPFS cluster
The value for URL must be changed to match your Kubernetes environment. The public bool defines if a load balancer should be created. This load balancer allows for ipfs gets to be done from systems outside of the Kubernetes environment.

```yaml
apiVersion: cluster.ipfs.io/v1alpha1
kind: Ipfs
metadata:
  name: ipfs-sample-1
spec:
  ipfsStorage: 2Gi
  clusterStorage: 2Gi
```
Once the values match your environment run the following.
```bash
kubectl create -n default -f ifps.yaml
```
