.PHONY: build test dev docker-up docker-down chaos-inject chaos-remove clean ui

# Build the Go binary (assumes web/dist exists)
build:
	@echo "📦 Building PulseMesh..."
	cd web && npm ci && npm run build
	go build -o bin/pulsemesh ./cmd/pulsemesh

# Run all tests with race detector
test:
	@echo "🧪 Running tests..."
	go test -v -race -count=1 ./...

# Run in dev mode (no embedded UI)
dev:
	go run ./cmd/pulsemesh --node-name=dev-node --http-port=8080 --gossip-port=7946 --probe-port=9000

# Build and start Docker cluster
docker-up:
	@echo "🐳 Starting 5-node PulseMesh cluster..."
	docker compose -f deploy/docker-compose.yml up --build -d
	@echo "✅ Cluster running. Dashboard at http://localhost:8080"

# Stop Docker cluster
docker-down:
	docker compose -f deploy/docker-compose.yml down -v

# Inject chaos on node-3
chaos-inject:
	@bash deploy/chaos.sh inject

# Remove chaos from node-3
chaos-remove:
	@bash deploy/chaos.sh remove

# Build UI only
ui:
	cd web && npm ci && npm run build

# Clean build artifacts
clean:
	rm -rf bin/ web/dist web/node_modules
