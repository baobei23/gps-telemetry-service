include .envrc
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

.PHONY: seed-users
seed-users:
	@go run db/seed/users/main.go

.PHONY: seed-products
seed-products:
	@go run db/seed/products/main.go

.PHONY: seed
seed: seed-users seed-products

.PHONY: gen-docs
gen-docs:
	@swag init -g ./main.go -d .,cmd,internal && swag fmt

.PHONY: e2e
e2e:
	@./scripts/e2e.sh
