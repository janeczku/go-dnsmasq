
VERSION ?= 1.7

.PHONY: build
build:
	go build -ldflags "-w -s -X main.Version=$(VERSION)" -tags="netgo" -trimpath .