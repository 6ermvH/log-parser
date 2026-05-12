SWAG_VERSION := v2.0.0-rc4

OPENAPI_DIR  := docs/openapi
OPENAPI_JSON := $(OPENAPI_DIR)/openapi.json
POSTMAN_FILE := docs/postman_collection.json

.PHONY: openapi postman docs lint test test-integration test-e2e mocks

openapi:
	go run github.com/swaggo/swag/v2/cmd/swag@$(SWAG_VERSION) init \
		--generalInfo doc.go \
		--dir internal/api/v1/http \
		--output $(OPENAPI_DIR) \
		--outputTypes json \
		--v3.1
	mv $(OPENAPI_DIR)/swagger.json $(OPENAPI_JSON)

postman: $(OPENAPI_JSON)
	npx --yes -p openapi-to-postmanv2 openapi2postmanv2 \
		-s $(OPENAPI_JSON) -o $(POSTMAN_FILE) -p

docs: openapi postman

mocks:
	go generate ./...

lint:
	golangci-lint run ./...

test:
	go test -count=1 -cover ./...

test-integration:
	go test -count=1 -tags integration -timeout 5m ./...

test-e2e:
	go test -count=1 -tags e2e -timeout 5m ./tests/e2e/...
