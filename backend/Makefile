.PHONY: build build-ws-handler build-http-api build-ws-authorizer clean tidy

DIST := dist
GOFLAGS := GOOS=linux GOARCH=arm64 CGO_ENABLED=0
BUILDFLAGS := -tags lambda.norpc -trimpath -ldflags="-s -w"

build: build-ws-handler build-http-api build-ws-authorizer

build-ws-handler:
	$(GOFLAGS) go build $(BUILDFLAGS) -o $(DIST)/ws-handler/bootstrap ./cmd/ws-handler/
	cd $(DIST)/ws-handler && zip -j ws-handler.zip bootstrap

build-http-api:
	$(GOFLAGS) go build $(BUILDFLAGS) -o $(DIST)/http-api/bootstrap ./cmd/http-api/
	cd $(DIST)/http-api && zip -j http-api.zip bootstrap

build-ws-authorizer:
	$(GOFLAGS) go build $(BUILDFLAGS) -o $(DIST)/ws-authorizer/bootstrap ./cmd/ws-authorizer/
	cd $(DIST)/ws-authorizer && zip -j ws-authorizer.zip bootstrap

clean:
	rm -rf $(DIST)

tidy:
	go mod tidy

run-local:
	USERS_TABLE=chatterchat-users \
	ROOMS_TABLE=chatterchat-rooms \
	MESSAGES_TABLE=chatterchat-messages \
	CONNECTIONS_TABLE=chatterchat-connections \
	LOCAL_DEV_USER="dev-001:alice:alice@local.dev" \
	go run ./cmd/local/
