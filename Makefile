# SPDX-License-Identifier: Apache-2.0
# Copyright 2021 Authors of KubeArmor

ifeq (, $(shell which govvv))
	$(shell go install github.com/ahmetb/govvv@latest)
endif

CURDIR     := $(shell pwd)
INSTALLDIR := $(shell go env GOPATH)/bin/
GIT_INFO   := $(shell govvv -flags -pkg $(shell go list ./version))

.PHONY: build
build:
	cd $(CURDIR); go mod tidy; CGO_ENABLED=0 go build -ldflags "-w -s ${GIT_INFO}" -o karmor

.PHONY: install
install: build
	install -m 0755 karmor $(DESTDIR)$(INSTALLDIR)
	
.PHONY: protobuf
vm-protobuf:
	cd $(CURDIR)/vm/protobuf; protoc --proto_path=. --go_opt=paths=source_relative --go_out=plugins=grpc:. vm.proto

.PHONY: gofmt
gofmt:
	cd $(CURDIR); gofmt -s -d $(shell find . -type f -name '*.go' -print)

.PHONY: golint
golint:
ifeq (, $(shell which golint))
	@{ \
	set -e ;\
	GOLINT_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOLINT_TMP_DIR ;\
	go mod init tmp ;\
	go get -u golang.org/x/lint/golint ;\
	rm -rf $$GOLINT_TMP_DIR ;\
	}
endif
	cd $(CURDIR); golint ./...

.PHONY: gosec
gosec:
ifeq (, $(shell which gosec))
	@{ \
	set -e ;\
	GOSEC_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOSEC_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/securego/gosec/v2/cmd/gosec ;\
	rm -rf $$GOSEC_TMP_DIR ;\
	}
endif
	cd $(CURDIR); gosec ./...
