SHELL := /bin/bash

HASURA_GRAPHQL_URL ?= http://localhost:8080/v1/graphql
HASURA_ADMIN_SECRET ?= hasura-admin-secret

.PHONY: up down logs gqlgen schema-fetch gqlgenc hasura-up gen-hasura-client api-test

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f --tail=200

hasura-up:
	docker compose up -d postgres hasura

gqlgen:
	cd api && go run github.com/99designs/gqlgen generate

schema-fetch:
	npx -y get-graphql-schema $(HASURA_GRAPHQL_URL) --header "x-hasura-admin-secret=$(HASURA_ADMIN_SECRET)" > api/internal/hasuragql/schema.graphql

gqlgenc:
	cd api/internal/hasuragql && go run github.com/maaft/gqlgenc

gen-hasura-client: schema-fetch gqlgenc

api-test:
	cd api && go test ./...
