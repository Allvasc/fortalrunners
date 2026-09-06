.PHONY: infra-up infra-down infra-logs backend web mobile

infra-up: ## sobe Postgres+PostGIS, Redis e NATS
	docker compose -f infra/docker-compose.yml up -d

infra-down: ## derruba a infra (mantém volumes)
	docker compose -f infra/docker-compose.yml down

infra-nuke: ## derruba a infra e apaga os dados
	docker compose -f infra/docker-compose.yml down -v

infra-logs:
	docker compose -f infra/docker-compose.yml logs -f

backend: ## atalho: entra em backend/ e roda a API
	$(MAKE) -C backend run

web:
	cd web && npm run dev

mobile:
	cd mobile && npx expo start

scheduler:
	$(MAKE) -C backend run-scheduler

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
