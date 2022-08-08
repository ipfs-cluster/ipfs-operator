# include version info
include ../version.mk

# image
IMAGE := ipfs-cluster-k8s-image


.PHONY: all
all: image

.PHONY: image
image:
	docker build \
		--build-arg "builddate_arg=$(BUILDDATE)" \
		--build-arg "version_arg=$(BUILD_VERSION)" \
		-t $(IMAGE) \
		-f Dockerfile .
