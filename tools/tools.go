//go:build tools

package tools

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=client.yaml -o ../internal/client/client.gen.go ../spec/api.yaml
