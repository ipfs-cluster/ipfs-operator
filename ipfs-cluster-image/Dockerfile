# Build the IPFS Cluster image using Golang
FROM golang:1.18 as builder
USER root

# create a workspace for the build
WORKDIR /workspace

# build from the latest release of IPFS Cluster
ARG IPFS_CLUSTER_VERSION="v1.0.2"
ARG IPFS_CLUSTER_GIT_HASH="b2ce7d916dd2828d86954dac98495eb3aebb12bf"

# just clone the repo at the most recent hash
RUN git clone \
	--depth 1 \
	--branch=${IPFS_CLUSTER_GIT_HASH} \
	https://github.com/ipfs-cluster/ipfs-cluster.git
WORKDIR /workspace/ipfs-cluster

# Ensure we have the correct IPFS Cluster release
RUN /bin/sh -c "[[ $(git rev-list -n 1 HEAD) == ${IPFS_CLUSTER_GIT_HASH} ]]"

# this will create binaries for the following programs under cmd:
# - cmd/ipfs-cluster-service
# - cmd/ipfs-cluster-ctl
# - cmd/ipfs-cluster-follow
RUN make build


# TODO: ensure FIPS is enabled

# Build the operator-specific image using our binaries
FROM registry.access.redhat.com/ubi9-minimal
RUN microdnf --refresh update -y && \
    microdnf clean all && \
    rm -rf /var/cache/yum

# TODO: set where volume mounts will be

# question: should protocols be tightly bound or will these change?
EXPOSE \ 
	# API HTTP port
	9094/tcp \
	# proxy HTTP port
	9095/udp \
	# cluster swarm port
	9096/tcp

# copy the built binaries from earlier
COPY --from=builder \
	/workspace/ipfs-cluster/cmd/ipfs-cluster-service \
	/usr/local/bin/ipfs-cluster-service
COPY --from=builder \
	/workspace/ipfs-cluster/cmd/ipfs-cluster-ctl \
	/usr/local/bin/ipfs-cluster-ctl
COPY --from=builder \
	/workspace/ipfs-cluster/cmd/ipfs-cluster-follow \
	/usr/local/bin/ipfs-cluster-follow

# image args
ARG builddate_arg="(unknown)"
ARG version_arg="(unknown)"
ENV builddate="${builddate_arg}"
ENV version="${version_arg}"

LABEL org.label-schema.build-date="${builddate}" \
			org.label-schema.description="IPFS Cluster image built for the IPFS Cluster Kubernetes Operator" \
			org.label-schema.license="Apache 2" \
			org.label-schema.name="ipfs-cluster-operator" \
			org.label-schema.schema-version="1.0" \
			org.label-schema.vcs-ref="${version}" \
			# the VCS URL and vendor name are subject to change
			org.label-schema.vcs-url="https://github.com/redhat-et/ipfs-operator" \
			org.label-schema.vendor="Red Hat, Inc." \ 
			org.label-schema.version="${version}"

# move the entry file into the root of the image
COPY entry.sh /
RUN chmod a+rx /entry.sh

ENTRYPOINT [ "/bin/bash" ]