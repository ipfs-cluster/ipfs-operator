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

### Running in KIND

An easy way to test and modify changes to the operator is by running it in a local KIND cluster.
To bootstrap a KIND cluster, you can run `hack/setup-kind-cluster.sh`, which will install all of the 
required components to operate an IPFS cluster. 

To deploy the operator in this repository into the cluster, you can run `hack/run-in-kind.sh` which
will build the source code and inject it into the cluster.
If you make subsequent changes, you will need to re-run `hack/run-in-kind.sh` and kill the previous
operator manager by running `kubectl delete pod -A -n ipfs-operator-system` in order to redploy the updated image. 

### Testing Local Changes

If you're developing the operator and would like to test your changes locally, you can do this by
running the kuttl end-to-end tests with `make test-e2e` after redploying the operator.

