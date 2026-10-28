# gmssh-cli-plugin — GMSSH CLI plugin scaffold
#
# Product layout (aligned with platform packaging):
#   dist/app/                      ← integration package (make integration)
#     uninstall.sh                 ← platform uninstall entry (next to app/)
#     app/bin/main                 ← CLI binary (host runs this)
#     data/config.json             ← runtime config
#     config/                      ← config templates (from configs/)
#     script/                      ← optional runtime scripts
#     logs/  tmp/
#
# Per-arch packages (make linux-amd64 / linux-arm64):
#   dist/linux-amd64/ ... same layout ...
#   dist/linux-arm64/ ...
#
# Shared recipes live under hack/*.mk; this file only sets variables and includes.

DIST_DIR      := dist
INTEGRATION   := $(DIST_DIR)/app
BACKEND_DIST  := $(DIST_DIR)/backend/main.dist

GO            ?= go
VERSION       ?= 0.1.0
CGO_ENABLED   ?= 0
export CGO_ENABLED

CMD_PKG       := ./cmd/main
LDFLAGS       := -s -w -X gmssh-cli-plugin/internal/biz/version.Number=$(VERSION)

include hack/check.mk
include hack/build.mk
include hack/bundle.mk
include hack/integration.mk
include hack/tar.mk
include hack/detect-os.mk
include hack/clean.mk
include hack/help.mk

.PHONY: all
all: check build-backend integration
