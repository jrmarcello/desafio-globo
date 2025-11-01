APP_NAME ?= votacao-paredao-bbb-api
WORKER_NAME ?= votacao-paredao-bbb-worker
API_CMD ?= ./cmd/api
WORKER_CMD ?= ./cmd/worker
BIN_DIR ?= bin
HTTP_PORT ?= 8080
RATE ?=
DURATION ?=
PRE_VUS ?=
MAX_VUS ?=
KIND_CLUSTER_NAME ?= votacao-paredao-bbb
KIND_CLUSTER_CONFIG ?= deploy/k8s/kind-cluster.yaml
K8S_NAMESPACE ?= votacao-paredao-bbb
API_IMAGE ?= votacao-paredao-bbb-api:latest
WORKER_IMAGE ?= votacao-paredao-bbb-worker:latest
POSTGRES_RELEASE ?= postgres
REDIS_RELEASE ?= redis

.PHONY: build build-worker run run-worker test tidy fmt vet lint docker-build docker-up docker-down logs logs-worker clean \
	kind-create kind-delete kind-build-images kind-load-images kind-namespace kind-deps kind-apply kind-rollout kind-smoke deploy-kind

build:
	go build -o $(BIN_DIR)/$(APP_NAME) $(API_CMD)

build-worker:
	go build -o $(BIN_DIR)/$(WORKER_NAME) $(WORKER_CMD)

run:
	go run $(API_CMD)

run-worker:
	go run $(WORKER_CMD)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path "./vendor/*")

vet:
	go vet ./...

lint: fmt vet

perf-prepare:
	docker compose up -d --build
	API_BASE=http://localhost:$(HTTP_PORT) ./tests/perf/setup.sh

perf-test: perf-prepare
	docker run --rm --network host \
	  -e API_BASE=http://localhost:$(HTTP_PORT) \
	  -e PAREDAO_ID=$$(grep PAREDAO_ID tests/perf/runtime.env | cut -d'=' -f2) \
	  -e PARTICIPANTE_IDS=$$(grep PARTICIPANTE_IDS tests/perf/runtime.env | cut -d'=' -f2) \
	  -e RATE=$(RATE) \
	  -e DURATION=$(DURATION) \
	  -e PRE_VUS=$(PRE_VUS) \
	  -e MAX_VUS=$(MAX_VUS) \
	  -v $$(pwd)/tests/perf:/scripts grafana/k6 run /scripts/k6-votos.js
	docker compose down -v
	rm -f tests/perf/runtime.env


docker-build:
	docker build -t $(APP_NAME):local .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-rebuild:
	docker compose up -d --build

logs:
	docker-compose logs -f api

logs-worker:
	docker-compose logs -f worker

docker-clean:
	docker-compose down -v --remove-orphans

clean:
	rm -rf $(BIN_DIR)

kind-create:
	kind create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CLUSTER_CONFIG)

kind-delete:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

kind-build-images:
	docker build -t $(API_IMAGE) -t $(WORKER_IMAGE) .

kind-load-images:
	kind load docker-image $(API_IMAGE) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(WORKER_IMAGE) --name $(KIND_CLUSTER_NAME)

kind-namespace:
	kubectl apply -f deploy/k8s/namespace.yaml

kind-deps: kind-namespace
	helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
	helm repo update >/dev/null
	helm upgrade --install $(POSTGRES_RELEASE) bitnami/postgresql -n $(K8S_NAMESPACE) \
		--set auth.username=bbb --set auth.password=bbb --set auth.database=bbb_votes
	helm upgrade --install $(REDIS_RELEASE) bitnami/redis -n $(K8S_NAMESPACE) \
		--set architecture=standalone --set auth.enabled=false

kind-apply:
	kubectl apply -f deploy/k8s/configmap.yaml
	kubectl apply -f deploy/k8s/deployment-api.yaml -f deploy/k8s/deployment-worker.yaml

kind-rollout:
	kubectl rollout status deployment/votacao-paredao-bbb-api -n $(K8S_NAMESPACE) --timeout=180s
	kubectl rollout status deployment/votacao-paredao-bbb-worker -n $(K8S_NAMESPACE) --timeout=180s

kind-smoke:
	kubectl delete pod curl-smoke -n $(K8S_NAMESPACE) --ignore-not-found
	kubectl run curl-smoke --restart=Never --namespace $(K8S_NAMESPACE) --image=curlimages/curl \
		--command -- /bin/sh -c "curl -sS -m 5 http://votacao-paredao-bbb-api.$(K8S_NAMESPACE).svc.cluster.local:8080/readyz"
	kubectl wait --for=condition=Ready --timeout=20s pod/curl-smoke -n $(K8S_NAMESPACE) >/dev/null 2>&1 || true
	kubectl logs curl-smoke -n $(K8S_NAMESPACE)
	kubectl delete pod curl-smoke -n $(K8S_NAMESPACE) --ignore-not-found

deploy-kind: kind-create kind-build-images kind-load-images kind-deps kind-apply kind-rollout kind-smoke
