# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: MPL-2.0

BINARY  := terraform-provider-ipam
VERSION ?= dev

.PHONY: build test testacc generate lint license-headers check-license-headers test-scripts docs

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test ./... -count=1

testacc:
	TF_ACC=1 go test ./... -count=1 -timeout 120m

generate:
	go generate ./...

lint:
	golangci-lint run
	./scripts/license-headers.sh --check

license-headers:
	./scripts/license-headers.sh

check-license-headers:
	./scripts/license-headers.sh --check

test-scripts:
	./scripts/test-license-headers.sh

TFPLUGINDOCS = go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

# Needs the terraform CLI (>= 1.11) on PATH.
docs:
	$(TFPLUGINDOCS) generate --provider-name ipam --rendered-provider-name IPAM
	$(TFPLUGINDOCS) validate --provider-name ipam
