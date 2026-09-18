.PHONY: generate update-spec build test test-integration

update-spec:
	cp ../platform/api.yaml spec/api.yaml

generate:
	cd tools && go generate -tags=tools ./...
	go run ./cmd/gen

build:
	go build -o dist/trustattic ./cmd/trustattic

test:
	go test ./...

test-integration:
	go test -tags=integration ./testing/... -v -timeout 5m
