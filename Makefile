MIGRATIONS_PATH = ./db/migrations
DB_ADDR=postgres://$(POSTGRES_USERNAME):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_STORENAME)?sslmode=$(POSTGRES_SSLMODE)

.PHONY: test
test:
	@go test -v ./...

.PHONY: migrate-create
migration:
	@migrate create -seq -ext sql -dir $(MIGRATIONS_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) up

.PHONY: migrate-down
migrate-down:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

.PHONY: test-race
test-race:
	@go test -v -race ./...

.PHONY: gen-docs
gen-docs:
	@swag init -g ./main.go -d . --outputTypes json,yaml && swag fmt

.PHONY: loadtest-ingest
loadtest-ingest:
	@k6 run scripts/loadtest/ingestion.js

.PHONY: loadtest-read
loadtest-read:
	@k6 run scripts/loadtest/read_api.js
