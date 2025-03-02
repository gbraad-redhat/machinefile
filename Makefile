BUILD_DIR ?= out

.PHONY: all
all: build

GOPATH ?= $(shell go env GOPATH)

BINARY_NAME := machinefile

DFLAGS := -extldflags='-static' -s -w $(GO_LDFLAGS)

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

$(BUILD_DIR)/linux-amd64/$(BINARY_NAME):
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $@ $(GO_BUILDFLAGS) ./cmd/machinefile/

$(BUILD_DIR)/linux-arm64/$(BINARY_NAME):
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $@ $(GO_BUILDFLAGS) ./cmd/machinefile/

.PHONY: cross ## Cross compiles all binaries
cross: $(BUILD_DIR)/linux-amd64/$(BINARY_NAME) $(BUILD_DIR)/linux-arm64/$(BINARY_NAME)

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(GO_BUILDFLAGS) ./cmd/machinefile/

