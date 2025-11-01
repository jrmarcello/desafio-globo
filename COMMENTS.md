# Considerações Gerais

## Decisões principais

- Mantive uma arquitetura Go enxuta separando API e worker. Escolhi Redis para fila e contadores porque ele resolve os dois problemas com uma dependência só: fila simples com `LPUSH/BRPOP`, contadores com `INCR` e latência baixa. Assim evitei trazer RabbitMQ/Kafka só para enfileirar votos.
- Comecei com SQLite, mas depois do `make perf-test` notei locks constantes e perda de throughput. Migrei para Postgres justamente pela concorrência robusta, suporte a migrations.
- Para coibir votos automatizados, adotei rate limit via Redis (IP + user-agent) antes de aceitar cada voto.
- Docker Compose virou escolha natural para desenvolvimento porque num comando só eu levanto Postgres, Redis e os binários Go. Pensando nos requisitos de automação e alta disponibilidade, implementei uma infra para Kubernetes com Kind: manifests simples e `make deploy-kind` permitem replicar o ambiente em um cluster kind.
- Criei o pipeline simples mas funcional de CI em GitHub Actions com lint, testes, race/coverage e um gatilho manual para carga. O CD local fica nos targets do Makefile (`deploy-kind`, `kind-delete`).
- Implementei a base de telemetria (logs estruturados, `/metrics`, `/readyz`), já pensando em evolução futura.
- Para o frontend, optei por páginas renderizadas com templates Go (SSR simples) servidas pela própria API.
- A migração inicial já cria um paredão de demonstração; com isso removi o endpoint público de criação e concentrei o fluxo em votação/consulta.

## Deploy, observabilidade e testes

- Setup local com Makefile + Docker Compose (`make docker-up`, `make perf-test`, `make docker-down`).
- CI em GitHub Actions + CD local com `make deploy-kind` para validar topologia em kind.
- Observabilidade: logs JSON (`docker compose logs`), `/readyz`, `/metrics` e servidor de métricas do worker escutando em `:9090`.

Passo a passo completo no README e no roteiro de testes (`docs/roteiro-testes.md`).

## Ideias e melhorias futuras

Para um cenário real com milhões de votos e picos agressivos de RPS, eu avaliaria estratégias adicionais como sharding de Postgres, uso de Redis Cluster ou até uma fila dedicada (Kafka) para maior throughput, além de replicar a API/worker em múltiplas zonas com um load balancer gerenciado. Além disso, pensando no tempo e complexidade, deixei algumas melhorias em backlog:

- Reforçar a infra Kubernetes com HA e melhorar a segurança: configurar HPA e revisar políticas NetworkPolicy implementando Policies que bloqueiem tudo e liberem só o necessário.
- Habilitar reCAPTCHA/hCaptcha e/ou outros mecanismos que ajudam a identificar padrões suspeitos de fraude (Robos) além do rate limit.
- Instrumentar métricas específicas para bloqueios antifraude e criar alertas.
