SHELL := /usr/bin/env bash

# github access token to build private module
GITHUB_TOKEN ?= unset

# will run all the required steps to run our gRPC server locally
# including the install-tools and build steps.
.PHONY: server
server:
	@echo "=== Running storefront-backend http server"
	docker-compose up -d

# rebuild storefront-server docker container.
.PHONY: rebuild
rebuild:
	@echo "=== Rebuilding storefront-backend http server"
	docker-compose up -d --build http-server

# build container image
.PHONY: image
image:
	@echo "=== Building storefront-backend http server container image"
	docker build \
		--build-arg GITHUB_TOKEN=$(GITHUB_TOKEN) \
		--build-arg VERSION=$$IMAGE_TAG \
		-t $$IMAGE_NAME .

# run unit tests and coverage
.PHONY: test
test:
	go test -cover -race -v -count=1 ./...
