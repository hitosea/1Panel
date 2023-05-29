GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOARCH=$(shell go env GOARCH)
GOOS=$(shell go env GOOS )

BASE_PAH := $(shell pwd)
BUILD_PATH = $(BASE_PAH)/build
WEB_PATH=$(BASE_PAH)/frontend
SERVER_PATH=$(BASE_PAH)/backend
MAIN= $(BASE_PAH)/cmd/server/main.go
APP_NAME=1panel

VERSION := $(shell git describe --tags --always --match="v*" 2> /dev/null || echo v0.0.0)
LDFLAGS	:= -s -w --extldflags "-fpic" -X "1Panel/cmd.PanelVersion=$(VERSION)"

build_frontend:
	cd $(WEB_PATH) && npm install && npm run build:dev

build_backend_on_linux:
	cd $(SERVER_PATH) \
    && CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -trimpath -ldflags '-s -w --extldflags "-static -fpic"' -tags 'osusergo,netgo' -o $(BUILD_PATH)/$(APP_NAME) $(MAIN)

build_backend_on_darwin:
	cd $(SERVER_PATH) \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ $(GOBUILD) -trimpath -ldflags '-s -w --extldflags "-static -fpic"'  -o $(BUILD_PATH)/$(APP_NAME) $(MAIN)

build_backend_on_archlinux:
	cd $(SERVER_PATH) \
    && CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -trimpath -ldflags '-s -w --extldflags "-fpic"' -tags osusergo -o $(BUILD_PATH)/$(APP_NAME) $(MAIN)

build_backend_release:
	cd $(SERVER_PATH) \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GOBUILD) -trimpath -ldflags '$(LDFLAGS)' -tags osusergo -o $(BASE_PAH)/$(APP_NAME)-$(VERSION)-linux-amd64/$(APP_NAME) $(MAIN)
	cd $(SERVER_PATH) \
    && CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc-10 $(GOBUILD) -trimpath -ldflags '$(LDFLAGS)' -tags osusergo -o $(BASE_PAH)/$(APP_NAME)-$(VERSION)-linux-arm64/$(APP_NAME) $(MAIN)
	@for arch in amd64 arm64; \
	do \
		tar zcf $(APP_NAME)-$(VERSION)-linux-$$arch.tar.gz $(APP_NAME)-$(VERSION)-linux-$$arch; \
	done

build_all: build_frontend  build_backend_on_linux
