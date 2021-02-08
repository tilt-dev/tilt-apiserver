.PHONY: vendor

all: test

# Run tests
test:
	go test ./... -mod vendor

# Run the apiserver locally
run-apiserver:
	go run ./cmd/apiserver/main.go --insecure-port=9443

vendor:
	go mod vendor

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: pkg/apis/core/v1alpha1/manifest_types.go hack/update-codegen.sh
	hack/update-codegen.sh

verify-generate:
	hack/verify-codegen.sh
