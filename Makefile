run-shell:
	@echo "Running shell..."
	go run shellapp/main.go

migrate:
	@echo "Running migrations..."
	go run broker/main.go --migrate-only

build-and-run-broker:
	@echo "Building and running broker..."
	scripts/build-and-run-broker.sh