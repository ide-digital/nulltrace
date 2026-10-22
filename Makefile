.PHONY: test build install

test:
	cd source && go test ./...

build:
	cd source && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../bin/nulltrace ./cmd/nulltrace

install:
	sh source/scripts/install.sh
