run-shell:
	@echo "Running shell..."
	go run shellapp/main.go

migrate:
	@echo "Running migrations..."
	go run broker/main.go --migrate-only