# Needs Go 1.23+ and a POSIX shell (Git Bash is fine on Windows).
#
#   make build                      everything for this machine, into bin/
#   make build GOOS=windows         bin/NullTrace.exe plus bin/app/ (as in the zip)
#   make test
#   make catalog                    probe every opt-out page (needs network)

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
export GOOS
export GOARCH
export CGO_ENABLED = 0

EXT    := $(if $(filter windows,$(GOOS)),.exe,)
FLAGS  := -trimpath -ldflags="-s -w"
OUT    := $(CURDIR)/bin

.PHONY: build test catalog install clean

# On Windows the layout matches the release zip. The launcher can't sit next
# to nulltrace.exe (Windows treats NullTrace.exe and nulltrace.exe as one file),
# so both engines live under app/.
ifeq ($(GOOS),windows)
DEST := $(OUT)/app
else
DEST := $(OUT)
endif

build:
	mkdir -p "$(DEST)"
	go build -C source $(FLAGS) -o "$(DEST)/nulltrace$(EXT)" ./cmd/nulltrace
	go build -C source $(FLAGS) -o "$(DEST)/nulltraced$(EXT)" ./cmd/nulltraced
ifeq ($(GOOS),windows)
	go build -C source -trimpath -ldflags="-H windowsgui -s -w" -o "$(OUT)/NullTrace.exe" ./cmd/launch
endif
	@echo "built into $(OUT)"

test:
	cd source && go vet ./... && go test ./...

catalog:
	cd source && go run ./cmd/catalogcheck -report-dir ../catalog-report

install:
	sh source/scripts/install.sh

clean:
	rm -rf bin catalog-report
