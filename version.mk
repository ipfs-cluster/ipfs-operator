# This file contains information about versioning that is used when building containers.

VERSION := 0.0.1
HEAD_HASH := $(shell git rev-parse --short HEAD)
# whether or not this repository has any commits, or this is directly from the commit#
DIRTY := $(shell git diff --quiet || echo '-dirty')
# the full version string, to be consumed by containers
BUILD_VERSION := v$(VERSION)+$(HEAD_HASH)$(DIRTY)

BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%S.%NZ')