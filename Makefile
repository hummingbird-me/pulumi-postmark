PACK         := postmark
ORG          := hummingbird-me
PROJECT      := github.com/$(ORG)/pulumi-$(PACK)
PROVIDER     := pulumi-resource-$(PACK)
VERSION      ?= 0.1.0
VERSION_PATH := $(PROJECT)/provider/version.Version

WORKING_DIR  := $(shell pwd)
SCHEMA_FILE  := $(WORKING_DIR)/provider/cmd/$(PROVIDER)/schema.json
GOSDK_MODULE := $(PROJECT)/sdk

PULUMI := pulumi

.PHONY: provider install schema gen_go_sdk gen_python_sdk gen_nodejs_sdk gen_dotnet_sdk gen_java_sdk build_sdks build lint test test_unit clean tidy ensure

# --- provider binary ---------------------------------------------------------

provider:
	cd provider && go build -o $(WORKING_DIR)/bin/$(PROVIDER) \
		-ldflags "-X $(VERSION_PATH)=$(VERSION)" \
		$(PROJECT)/provider/cmd/$(PROVIDER)

install: provider
	cp $(WORKING_DIR)/bin/$(PROVIDER) $(shell go env GOPATH)/bin/$(PROVIDER)

# --- schema ------------------------------------------------------------------

# Emit the checked-in schema.json from the freshly built binary (version stripped
# so the file is stable across releases). CI should `git diff --exit-code` this.
schema: provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/$(PROVIDER) | jq 'del(.version)' > $(SCHEMA_FILE)

# --- SDKs --------------------------------------------------------------------

gen_go_sdk: provider
	rm -rf sdk/go
	$(PULUMI) package gen-sdk --language go $(WORKING_DIR)/bin/$(PROVIDER) --out sdk --version $(VERSION)
	cd sdk && (test -f go.mod || go mod init $(GOSDK_MODULE)) && go mod tidy

gen_python_sdk: provider
	rm -rf sdk/python
	$(PULUMI) package gen-sdk --language python $(WORKING_DIR)/bin/$(PROVIDER) --out sdk --version $(VERSION)

gen_nodejs_sdk: provider
	rm -rf sdk/nodejs
	$(PULUMI) package gen-sdk --language nodejs $(WORKING_DIR)/bin/$(PROVIDER) --out sdk --version $(VERSION)

gen_dotnet_sdk: provider
	rm -rf sdk/dotnet
	$(PULUMI) package gen-sdk --language dotnet $(WORKING_DIR)/bin/$(PROVIDER) --out sdk --version $(VERSION)

gen_java_sdk: provider
	rm -rf sdk/java
	$(PULUMI) package gen-sdk --language java $(WORKING_DIR)/bin/$(PROVIDER) --out sdk --version $(VERSION)

build_sdks: gen_go_sdk gen_python_sdk gen_nodejs_sdk gen_dotnet_sdk gen_java_sdk

build: provider schema build_sdks

# --- quality -----------------------------------------------------------------

tidy:
	cd provider && go mod tidy

lint:
	cd provider && go vet ./... && gofmt -l .

test_unit:
	cd provider && go test ./...

test: test_unit

clean:
	rm -rf bin

ensure: tidy
