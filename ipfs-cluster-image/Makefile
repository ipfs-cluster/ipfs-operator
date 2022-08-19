# include version info
include ../version.mk

# image
IMAGE := quay.io/redhat-et-ipfs/ipfs-cluster


.PHONY: all
all: image

.PHONY: image
image:
	docker build \
		--build-arg "builddate_arg=$(BUILDDATE)" \
		--build-arg "version_arg=$(BUILD_VERSION)" \
		-t $(IMAGE) \
		-f Dockerfile .
