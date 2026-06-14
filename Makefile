PROJECT_NAME := Pulumi Postmark Provider

PACK             := postmark
PACKDIR          := sdk
PROJECT          := github.com/hummingbird-me/pulumi-postmark
NODE_MODULE_NAME := @kitsu-io/pulumi-postmark
NUGET_PKG_NAME   := HummingbirdMe.Postmark

PROVIDER        := pulumi-resource-${PACK}
PROVIDER_PATH   := provider
VERSION_PATH    := ${PROVIDER_PATH}.Version

PULUMI          := pulumi

SCHEMA_FILE     := provider/cmd/pulumi-resource-${PACK}/schema.json
export GOPATH   := $(shell go env GOPATH)

WORKING_DIR     := $(shell pwd)
TESTPARALLELISM := 4

# Override during CI using `make [TARGET] PROVIDER_VERSION=""` or by setting a PROVIDER_VERSION environment variable.
# Local & branch builds just use this fixed default version unless specified.
PROVIDER_VERSION ?= 1.0.0-alpha.0+dev
# Use this normalised version everywhere rather than the raw input to ensure consistency.
VERSION_GENERIC = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)")

# Pick up locally pinned pulumi-language-* plugins.
export PULUMI_IGNORE_AMBIENT_PLUGINS = true

ensure::
	go mod tidy

$(SCHEMA_FILE): provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/${PROVIDER} | \
		jq 'del(.version)' > $(SCHEMA_FILE)

# Codegen generates the schema file and *generates* all SDK sources. This is a local
# process that does not require the ability to compile the SDKs. To compile them, use
# `make build_sdks`.
codegen: $(SCHEMA_FILE) sdk/dotnet sdk/go sdk/nodejs sdk/python sdk/java

.PHONY: sdk/%
sdk/%: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language $* $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/python: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md ${PACKDIR}/python/

sdk/go: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language go $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp go.mod ${PACKDIR}/go/pulumi-${PACK}/go.mod
	cd ${PACKDIR}/go/pulumi-${PACK} && \
		go mod edit -module=${PROJECT}/${PACKDIR}/go/pulumi-${PACK} && \
		go mod tidy

PROVIDER_SRC := $(shell find provider -type f -name '*.go') go.mod go.sum

.PHONY: provider
provider: bin/${PROVIDER}

bin/${PROVIDER}: $(PROVIDER_SRC)
	cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} \
		-ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" \
		$(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER)

.PHONY: provider_debug
provider_debug:
	cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -gcflags="all=-N -l" \
		-ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" \
		$(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER)

test_provider:
	go test -short -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./provider/...

dotnet_sdk: sdk/dotnet
	cd ${PACKDIR}/dotnet/ && \
		echo "${VERSION_GENERIC}" > version.txt && \
		dotnet build

go_sdk: sdk/go

nodejs_sdk: sdk/nodejs
	cd ${PACKDIR}/nodejs/ && \
		yarn install && \
		yarn run tsc
	cp README.md LICENSE ${PACKDIR}/nodejs/package.json ${PACKDIR}/nodejs/yarn.lock ${PACKDIR}/nodejs/bin/

python_sdk: sdk/python
	cp README.md ${PACKDIR}/python/
	cd ${PACKDIR}/python/ && \
		rm -rf ./bin/ ../python.bin/ && cp -R . ../python.bin && mv ../python.bin ./bin && \
		python3 -m venv venv && \
		./venv/bin/python -m pip install build && \
		cd ./bin && \
		../venv/bin/python -m build .

java_sdk:: PACKAGE_VERSION := $(VERSION_GENERIC)
java_sdk:: sdk/java
	cd sdk/java/ && \
		gradle --console=plain build

.PHONY: build
build:: provider build_sdks

.PHONY: build_sdks
build_sdks: dotnet_sdk go_sdk nodejs_sdk python_sdk java_sdk

lint:
	golangci-lint --config .golangci.yml run --fix

install:: install_nodejs_sdk
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin

GO_TEST := go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM}

test:: test_provider
	cd examples/simple && go build ./...

install_dotnet_sdk::
	rm -rf $(WORKING_DIR)/nuget/$(NUGET_PKG_NAME).*.nupkg
	mkdir -p $(WORKING_DIR)/nuget
	find . -name '*.nupkg' -print -exec cp -p {} ${WORKING_DIR}/nuget \;

install_python_sdk::
	#target intentionally blank

install_go_sdk::
	#target intentionally blank

install_java_sdk::
	#target intentionally blank

install_nodejs_sdk::
	-yarn unlink --cwd $(WORKING_DIR)/sdk/nodejs/bin
	yarn link --cwd $(WORKING_DIR)/sdk/nodejs/bin
