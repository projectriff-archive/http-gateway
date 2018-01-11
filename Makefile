.PHONY: build build-for-docker clean dockerize
OUTPUT = http-gateway
OUTPUT_LINUX = http-gateway-linux
BUILD_FLAGS =

ifeq ($(OS),Windows_NT)
    detected_OS := Windows
else
    detected_OS := $(shell sh -c 'uname -s 2>/dev/null || echo not')
endif

ifeq ($(detected_OS),Linux)
        BUILD_FLAGS += -ldflags "-linkmode external -extldflags -static"
endif

GO_SOURCES = $(shell find pkg cmd -type f -name '*.go')

build: $(OUTPUT)

build-for-docker: $(OUTPUT_LINUX)

test: build
	go test -v ./...

.arch-linux: vendor

$(OUTPUT): $(GO_SOURCES) vendor
	go build -o http-gateway cmd/*.go

$(OUTPUT_LINUX): $(GO_SOURCES) vendor
	# This builds the executable from Go sources on *your* machine, targeting Linux OS
	# and linking everything statically, to minimize Docker image size
	# See e.g. https://blog.codeship.com/building-minimal-docker-containers-for-go-applications/ for details
	CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS) -v -a -installsuffix cgo -o $(OUTPUT_LINUX) cmd/*.go

vendor: Gopkg.toml
	dep ensure

clean:
	rm -f $(OUTPUT)
	rm -f $(OUTPUT_LINUX)

dockerize: build-for-docker
	docker build . -t projectriff/http-gateway:0.0.3-snapshot
