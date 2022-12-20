## These are some common variables for Make

PROJECT_ROOT_DIR = .
PROJECT_NAME ?= appuio-cloud-reporting
PROJECT_OWNER ?= appuio

## BUILD:go
BIN_FILENAME ?= $(PROJECT_NAME)

## BUILD:docker
DOCKER_CMD ?= docker

IMG_TAG ?= latest
# Image URL to use all building/pushing image targets
CONTAINER_IMG ?= local.dev/$(PROJECT_OWNER)/$(PROJECT_NAME):$(IMG_TAG)

## COMPOSE:
COMPOSE_CMD ?= docker-compose
COMPOSE_DB_URL ?= postgres://reporting:reporting@localhost:55432/reporting-db?sslmode=disable
COMPOSE_FILE ?= docker-compose.yml

PROMETHEUS_VERSION ?= 2.40.7
PROMETHEUS_DIST ?= $(shell go env GOOS)
PROMETHEUS_ARCH ?= $(shell go env GOARCH)
PROMETHEUS_DOWNLOAD_LINK ?= https://github.com/prometheus/prometheus/releases/download/v$(PROMETHEUS_VERSION)/prometheus-$(PROMETHEUS_VERSION).$(PROMETHEUS_DIST)-$(PROMETHEUS_ARCH).tar.gz
