CURDIR=$(shell pwd)

.PHONY: build
build:
	cd $(CURDIR); go mod tidy; go build -o karmor