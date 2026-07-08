.PHONY: build test frontend docker deploy

build:
	go build -o bin/genesisrk .

test:
	go test ./internal/export/... ./internal/generator/... -count=1

frontend:
	cd frontend && npm ci && npm run build

docker:
	docker build -f deploy/azure/Dockerfile.genesis -t genesisrk:latest .

deploy:
	cd deploy/azure && ./container-app.sh all
