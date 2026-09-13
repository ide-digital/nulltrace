.PHONY: test build

test:
	cd source && go test ./...

build:
	cd source && powershell -File build.ps1
