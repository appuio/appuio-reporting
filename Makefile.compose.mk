.PHONY: docker-compose-up
docker-compose-up: ## Starts up docker compose services
	@$(COMPOSE_CMD) -f $(COMPOSE_FILE) $(compose_args) up --detach
	@echo ''
	@echo '  To connect to the DB:'
	@echo '  $$ psql postgres://reporting:reporting@localhost/appuio-cloud-reporting-test'
	@echo ''

.PHONY: docker-compose-down
docker-compose-down: ## Stops docker compose services
	@$(COMPOSE_CMD) -f $(COMPOSE_FILE) $(compose_args) down

.PHONY: ping-postgres
ping-postgres: docker-compose-up ## Waits until postgres is ready to accept connections
	$(COMPOSE_CMD) -f $(COMPOSE_FILE) $(compose_args) exec -T -- postgres sh -c "until pg_isready; do sleep 1s; done; sleep 1s"
