FROM docker.io/ipfs/ipfs-cluster:1.0.2

# question: should protocols be tightly bound or will these change?
EXPOSE \ 
	# API HTTP port
	9094 \
	# proxy HTTP port
	9095 \
	# cluster swarm port
	9096


# image args
ARG builddate_arg="(unknown)"
ARG version_arg="(unknown)"
ENV builddate="${builddate_arg}"
ENV version="${version_arg}"

# TODO: change these accordingly
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

ENTRYPOINT [ "/bin/sh" ]