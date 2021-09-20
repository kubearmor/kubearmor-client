CURDIR=$(shell pwd)
VERSION=$(shell git describe --tags --dirty --always)

.PHONY: build
build:
	cd $(CURDIR)
	go mod tidy
	CGO_ENABLED=0 go build \
	-ldflags "-w -s -X github.com/kubearmor/kubearmor-client/version.version=${VERSION}" \
	-o karmor