# Desafio BBB — Plano e execução

Projeto em Go que simula a votação do paredão do BBB: API enxuta, processamento assíncrono com Redis/Postgres, antifraude básico e automações para rodar localmente, em Docker ou em um cluster kind. O plano completo está em `docs/plano-execucao.md`; decisões e pendências ficam registradas no `COMMENTS.md`.

## Pré-requisitos

- Go 1.25+
- Docker + Docker Compose
- Make
- (Opcional para Kubernetes):
  - kind (cluster local)
  - helm (gerenciador de dependências)

Crie seu arquivo de configuração:

```bash
cp .env.example .env
```

## Como rodar

### Docker Compose

```bash
make docker-up    # sobe API, worker, Postgres e Redis
make logs         # acompanha os logs da API
make logs-worker  # acompanha o worker
make docker-down  # encerra o stack
make docker-clean # remove containers, volumes e redes criados
```

As migrations rodam automaticamente quando a API inicia (via gormigrate).

### Testes e Observabilidade

- `go test ./...` para a suíte unitária.
- `make perf-test` roda o cenário de carga com k6 (~1000 req/s por 30s).
- Endpoints prontos: `/healthz`, `/readyz` e `/metrics`. O worker expõe métricas se `WORKER_METRICS_ADDRESS` estiver setado (default `:9090`).

## Frontend web (SSR simplificado)

- A API serve páginas HTML com templates Go (`internal/app/web`).
- Fluxos principais:
  - `/vote`: lista o paredão ativo e permite enviar o voto (POST). Após o envio, o usuário é redirecionado para `/panorama`.
  - `/panorama`: mostra o comprovante, parciais por participante e totais por hora do paredão selecionado.
  - `/consulta`: painel para a produção com agregados; exige o token definido em `CONSULTA_TOKEN` (adicione no `.env`). Depois da validação, a página fica liberada via cookie `consulta-auth`.
- Para validar manualmente: `docker compose up -d --build` e acesse diretamente `/vote` (um paredão de demonstração é criado automaticamente na migração). Se desejar outro paredão, ajuste a seed ou insira manualmente via banco.

### Antifraude

O rate limit em Redis fica ativo por padrão (`ANTIFRAUDE_RATE_LIMIT_ENABLED=true`). Ajuste os parâmetros `ANTIFRAUDE_RATE_LIMIT_MAX` e `ANTIFRAUDE_RATE_LIMIT_WINDOW` conforme necessário; defina `false` para desabilitar durante testes.

## Kubernetes (opcional)

Temos manifests simples em `deploy/k8s/` pensados para um cluster kind com Postgres/Redis provisionados via Helm.

```bash
make deploy-kind   # cria cluster, instala dependências, builda e aplica manifests, roda smoke test
make kind-delete   # remove o cluster
```

## CI/CD

O workflow `CI` (GitHub Actions) executa lint (`golangci-lint`), testes, build e um job opcional de cargas (`make perf-test` via `workflow_dispatch`). A cobertura é publicada como artefato (`coverage.out`). Targets de CD local (`make deploy-kind`) servem como base para evoluir em direção ao publish das imagens/manifestos.
