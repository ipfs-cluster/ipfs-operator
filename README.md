# IPFS Operator
This operator is still heavily in progress.

# Running

```
make manifests
make generate
make test
make build
make build-push
make docker-build
make docker-push
make bundle
make bundle-build
make bundle-push



sudo podman stop microshift 
sudo podman volume rm microshift-data
sudo podman run -d --rm --name microshift --privileged -v microshift-data:/var/lib -p 6443:6443 quay.io/microshift/microshift-aio:latest
sleep 30
sudo podman cp microshift:/var/lib/microshift/resources/kubeadmin/kubeconfig ~/kubeconfig && sudo chown $(id -u): ~/kubeconfig
operator-sdk olm install
sleep 30
kubectl patch storageclass kubevirt-hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
kubectl create ns ipfs-operator-system
operator-sdk run bundle quay.io/rcook/ipfs-operator-bundle:v0.0.1 -n ipfs-operator-system
kubectl create -f config/samples/cluster_v1alpha1_ipfs.yaml
```
