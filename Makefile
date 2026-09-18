.PHONY: generate update-spec build test

update-spec:
	cp ../platform/api.yaml spec/api.yaml

generate:
	cd tools && go generate -tags=tools ./...

build:
	go build -o dist/trustattic ./cmd/trustattic

test:
	go test ./...
