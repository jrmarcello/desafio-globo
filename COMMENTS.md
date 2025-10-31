# Registro do desenvolvimento

- Dia 0: Setup inicial do projeto Go, criação do módulo `github.com/marcelojr/desafio-globo` e estrutura básica com servidor HTTP mínimo, endpoint `/healthz` para facilitar os testes iniciais.
- Dia 0: Configurei tooling com Makefile, Dockerfile e docker-compose incluindo Postgres e Redis; deixei targets para build/run/test e comandos de orquestração para acelerar o ciclo local.
- Dia 0: Tentativa de rodar `go mod tidy` falhou por falta de acesso à rede no ambiente atual; manterei essa etapa pendente até liberar rede ou adicionar dependências manuais.
- Dia 0: Modelei entidades e portas de domínio (interfaces para repositórios, fila, contador, antifraude) e subi um esqueleto inicial da camada de serviço de votação com validações básicas; métodos ainda retornam erros de "a implementar".
- Dia 0: Criei placeholders dos repositórios Postgres e infraestrutura Redis (contador em memória e fila simulada) mais a primeira migração SQL com tabelas de paredão, participante e voto, preparando terreno para implementação real.
- Dia 0: Modelei camada HTTP inicial com endpoints básicos para paredões/votos, injetei serviços placeholders no `main.go` e criei implementações temporárias de clock e antifraude para poder avançar sem dependências externas imediatas.
- Dia 1: Configurei carregamento de variáveis (`internal/platform/config`), conectores reais para Postgres via pgx e Redis via go-redis; substituí mocks da API por clientes de verdade e rodei `go mod tidy` com as novas dependências, mantendo a decisão de usar ULID em vez de UUID sequencial puro para as PKs.
- Dia 1: Atendi o pedido de usar GORM: adaptei a conexão com Postgres, repositórios de paredão/participante/voto e executei `go mod tidy` para trazer gorm.io/driver/postgres e gorm.io/gorm.
- Dia 1: Implementei serviço de domínio completo com geração de ULIDs, regras de período ativo, antifraude e contadores Redis; handlers HTTP agora recebem dados reais (parse de datas) e retornam os objetos criados, tudo com suporte ao novo gerador em `internal/platform/ids`.
- Dia 1: `go build ./...` rodou com sucesso após as alterações para garantir que não deixei o projeto quebrado.
- Dia 1: Ajustei o fluxo de votos para ser assíncrono (API só valida e enfileira), criei o worker em `cmd/worker` para consumir a fila Redis, persistir no Postgres via GORM e atualizar contadores; atualizei Dockerfile, docker-compose, Makefile e README com o novo componente.
- Dia 1: Adicionei testes unitários para o serviço (`internal/app/voting`) cobrindo criação de paredão, enfileiramento e cálculo de parciais, além de teste dedicado ao worker (`internal/app/worker`), com stubs em memória para fila/contador/repos e clock determinístico; `go test ./...` passando.
- Dia 1: Implementei antifraude real com rate limit em Redis (`ANTIFRAUDE_RATE_LIMIT_*`), integrando na API, tratando erro 429 e validando com testes usando miniredis; atualizei docs e `.env.example`.
- Dia 1: Validação manual: subi o stack (`docker compose up -d --build`) usando `HTTP_PORT=18080`, rodei migração e confirmei o rate limit (HTTP 202, 202, 429) com chamadas via `curlimages/curl` no mesmo IP/UA; derrubei o ambiente com `docker compose down` ao final.
- Dia 1: Adicionei pipeline de CI (`.github/workflows/ci.yml`) rodando gofmt (verificação), `go vet`, `go test ./...` e `go build ./...` em cada push/pull request, reforçando a automação antes de qualquer deploy.
- Dia 1: Deixei uma pastinha `deploy/k8s/` com manifests opcionais (namespace, config, secrets, API/worker e service) mais um README rápido explicando como aplicar em kind ou outro cluster; fica como referência extra, sem impactar quem só usa Docker Compose.
- Dia 1: Testei esse fluxo opcional na prática (`kind create cluster --name votacao-paredao-bbb`, helm para Postgres/Redis, `kind load docker-image`, `kubectl apply ...`, migração via `kubectl exec`, requisições com `kubectl run curl`) e, após validar as respostas 200/202, derrubei o cluster para não deixar resíduo.
- Dia 1: Padronizei nomenclatura das imagens/serviços para `votacao-paredao-bbb-*` (docker-compose e manifests k8s) para refletir melhor o domínio do projeto e evitar resíduos de nomes antigos.
- Dia 1: Ativei migrations automáticas via gormigrate (`DB_AUTO_MIGRATE=true` por padrão apenas na API; worker fica desabilitado para evitar concorrência). O SQL em `migrations/0001_init.sql` permanece como fallback manual.
- Dia 1: Testei o fluxo completo no Kubernetes com o cluster kind (1 control-plane + 2 workers),
  instalando Postgres/Redis via Helm, carregando as imagens, aplicando os manifests, verificando logs e
  validando criação de paredão/voto via `kubectl run curl`; ao final, derrubei o cluster.
- Dia 1: Adicionei PodDisruptionBudget para API e worker (`deploy/k8s/pdb-*.yaml`), garantindo que
  pelo menos uma réplica permaneça ativa durante manutenções planejadas.
- Dia 1: Criei o cenário de carga (`make perf-test` com k6 a ~1000 req/s); última execução local: 30k
  votos em 30s, p95 ≈ 1.15ms, sem erros. Script e rotina documentados em `docs/roteiro-testes.md`.
- Dia 1: Ajustei o setup Kubernetes opcional para usar um cluster kind com 1 control-plane e 2 workers (arquivo `deploy/k8s/kind-cluster.yaml`) e aumentei réplicas da API/worker para 2, reforçando o objetivo de HA.
- Dia 1: Criei alvos no Makefile (`deploy-kind`, `kind-delete`, etc.) para automatizar o fluxo local de CD com kind (cluster, helm deps, build/load, apply e smoke test), mantendo os passos manuais documentados.
- Dia 1: Ampliei o pipeline de CI com cache de módulos, `golangci-lint`, job separado de race/coverage e um gatilho manual (`workflow_dispatch`) que executa `make perf-test` e publica o resultado do k6.
- Ideia futura: instrumentar métricas/logs específicos para bloqueios do antifraude, por exemplo um counter Prometheus `antifraude_rate_limit_blocks_total`, para enxergar picos e acionar alertas sem depender só de logs.
- Ideia futura: reforçar o antifraude com uma etapa de verificação humana (reCAPTCHA/hCaptcha) e heurísticas adicionais (fingerprint/score), mitigando bots além do rate-limit atual.
- Ideia futura: evoluir os manifests opcionais para um workflow completo Kubernetes/Helm (rollouts, auto scale, jobs de migração) e publicar imagens em registry público para evitar `kind load`.
- Ideia futura: migrar para migrations automáticas com o GORM (ex.: `AutoMigrate` ou scripts versionados) e dispará-las via job ou na inicialização controlada, eliminando o passo manual do `kubectl exec`.
- Ideia futura: integrar o artefato de cobertura gerado no CI (coverage.out) a uma ferramenta como Codecov/Sonar para acompanhar a evolução ao longo do tempo.
- Ideia futura: evoluir para IaC (Terraform/Ansible) provisionando infraestrutura gerenciada (Redis/Postgres), namespaces e secrets, tudo integrado ao pipeline.
- Ideia futura: criar smoke tests pós-deploy (curl/k6) disparados automaticamente para garantir que os endpoints principais respondem antes de liberar um release.
