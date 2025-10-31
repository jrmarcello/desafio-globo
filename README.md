# Desafio BBB — Plano e execução

Projeto em Go que simula a votação do paredão do BBB: API enxuta, processamento assíncrono com Redis/Postgres, antifraude básico e automações para rodar localmente, em Docker ou em um cluster kind. O plano completo está em `docs/plano-execucao.md`; decisões e pendências ficam registradas no `COMMENTS.md`.

## Pré-requisitos rápidos

- Go 1.25+
- Docker + Docker Compose
- Make

Crie seu arquivo de configuração:

```bash
cp .env.example .env
```

As migrations rodam automaticamente quando a API inicia (via gormigrate). Se preferir acionar o SQL “na mão”, utilize:

```bash
cat migrations/0001_init.sql | docker compose exec -T postgres psql -U ${POSTGRES_USER:-bbb} -d ${POSTGRES_DB:-bbb_votes}
```

## Como rodar

### Go puro

```bash
make tidy   # organizar dependências (execute quando tiver rede)
make build  # compila API em bin/votacao-paredao-bbb-api
make run    # sobe a API em :8080
make run-worker  # em outro terminal, processa a fila
```

### Docker Compose

```bash
make docker-up    # sobe API, worker, Postgres e Redis
make logs         # acompanha os logs da API
make logs-worker  # acompanha o worker
make docker-down  # encerra o stack
```

Use `HTTP_PORT` no `.env` para mudar a porta exposta (padrão 8080).

### Observabilidade e testes

- `go test ./...` para a suíte unitária.
- `make perf-test` roda o cenário de carga com k6 (~1000 req/s por 30s).
- Endpoints prontos: `/healthz`, `/readyz` e `/metrics`. O worker expõe métricas se `WORKER_METRICS_ADDRESS` estiver setado (default `:9090`).

## Frontend web (SSR simplificado)

- A API serve páginas HTML com templates Go (`internal/app/web`).
- Fluxos principais:
  - `/vote`: lista o paredão ativo e permite enviar o voto (POST). Após o envio, o usuário é redirecionado para `/panorama`.
  - `/panorama`: mostra o comprovante, parciais por participante e totais por hora do paredão selecionado.
  - `/consulta`: painel para a produção com agregados; exige o token definido em `CONSULTA_TOKEN` (adicione no `.env`). Depois da validação, a página fica liberada via cookie `consulta-auth`.
- Para validar manualmente: `docker compose up -d --build`, crie um paredão (`POST /paredoes`) e navegue nas rotas acima (via navegador ou `curl` dentro da rede `desafio-globo_default`).

### Antifraude

O rate limit em Redis fica ativo por padrão (`ANTIFRAUDE_RATE_LIMIT_ENABLED=true`). Ajuste os parâmetros `ANTIFRAUDE_RATE_LIMIT_MAX` e `ANTIFRAUDE_RATE_LIMIT_WINDOW` conforme necessário; defina `false` para desabilitar durante testes.

## Kubernetes (opcional)

Temos manifests simples em `deploy/k8s/` pensados para um cluster kind com Postgres/Redis provisionados via Helm.

Atalho completo:

```bash
make deploy-kind   # cria cluster, instala dependências, builda e aplica manifests, roda smoke test
make kind-delete   # remove o cluster
```

Se preferir executar passo a passo:

1. `kind create cluster --name votacao-paredao-bbb --config deploy/k8s/kind-cluster.yaml`
2. `kubectl apply -f deploy/k8s/namespace.yaml`
3. Instale Postgres e Redis (`helm install postgres ...`, `helm install redis ...`)
4. `kind load docker-image votacao-paredao-bbb-api:latest --name votacao-paredao-bbb`
5. `kind load docker-image votacao-paredao-bbb-worker:latest --name votacao-paredao-bbb`
6. `kubectl apply -f deploy/k8s/configmap.yaml`
7. `kubectl apply -f deploy/k8s/deployment-api.yaml -f deploy/k8s/deployment-worker.yaml`
8. `kubectl run curl --rm --restart=Never -n votacao-paredao-bbb --image=curlimages/curl -- curl -sS http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/readyz`
9. `kind delete cluster --name votacao-paredao-bbb` para limpar quando terminar.

## CI/CD

O workflow `CI` (GitHub Actions) executa lint (`golangci-lint`), testes, build e um job opcional de cargas (`make perf-test` via `workflow_dispatch`). A cobertura é publicada como artefato (`coverage.out`). Targets de CD local (`make deploy-kind`) servem como base para evoluir em direção ao publish das imagens/manifestos.

## Referências rápidas

- Plano de execução: `docs/plano-execucao.md`
- Registro de decisões e resumo de deploy/testes: `COMMENTS.md`
- Roteiro de testes manuais (Docker + k6 + Kubernetes): `docs/roteiro-testes.md`
