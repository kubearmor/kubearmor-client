CURDIR=$(shell pwd)
INSTALLDIR=$(shell go env GOPATH)/bin/

ifeq (,$(shell which govvv))
$(shell go install github.com/ahmetb/govvv@latest)
endif

PKG := $(shell go list ./version)
GIT_INFO := $(shell govvv -flags -pkg $(PKG))

.PHONY: build
build:
	cd $(CURDIR)
	go mod tidy
	CGO_ENABLED=0 go build \
	-ldflags "-w -s ${GIT_INFO}" \
	-o karmor

.PHONY: install
install: build
	install -m 0755 karmor $(DESTDIR)$(INSTALLDIR)
	
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
