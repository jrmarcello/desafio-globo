# AGENTS.md

**Instruções para Agentes de IA**

Este arquivo contém instruções, contexto e diretrizes para agentes de IA (GitHub Copilot, ChatGPT, Claude, etc.) trabalharem efetivamente neste projeto. Leia todo este arquivo antes de fazer qualquer sugestão ou modificação.

---

## 📋 Índice

- [Visão Geral do Projeto](#visão-geral-do-projeto)
- [Arquitetura e Estrutura](#arquitetura-e-estrutura)
- [Stack Técnica](#stack-técnica)
- [Padrões e Convenções](#padrões-e-convenções)
- [Restrições Importantes](#restrições-importantes)
- [Fluxo de Desenvolvimento](#fluxo-de-desenvolvimento)
- [Comandos Essenciais](#comandos-essenciais)
- [Áreas Sensíveis](#áreas-sensíveis)
- [Diretrizes de Código](#diretrizes-de-código)

---

## Visão Geral do Projeto

**Nome:** Sistema de Votação do Paredão BBB  
**Objetivo:** Processar votos em tempo real com alta performance (1000 req/s baseline)  
**Linguagem:** Go 1.25+  
**Padrão Arquitetural:** Ports & Adapters (Hexagonal Architecture)

### Funcionalidades Principais

1. **Votação (`/vote`)**: Interface web para votar em participantes do paredão
2. **Panorama (`/panorama`)**: Resultados em tempo real com auto-refresh
3. **Consulta (`/consulta`)**: Painel administrativo protegido por token
4. **Antifraude**: Rate limiting por IP + User-Agent via Redis
5. **Processamento Assíncrono**: Worker consumindo fila Redis

---

## Arquitetura e Estrutura

### Arquitetura do Projeto

Este é um sistema de votação para paredão do BBB desenvolvido em Go. A arquitetura segue o padrão de Ports & Adapters (Hexagonal):

```plaintext
cmd/
├── api/       → Servidor HTTP principal (porta 8080)
└── worker/    → Processador assíncrono de votos (consome fila Redis)

internal/
├── app/
│   ├── httpapi/      → Handlers HTTP e rotas
│   ├── voting/       → Lógica de negócio de votação
│   ├── web/          → Frontend SSR (templates Go)
│   └── worker/       → Processador de votos
├── domain/
│   ├── models.go     → Entidades (Paredao, Participante, Voto)
│   ├── ports.go      → Interfaces (repositórios, serviços)
│   └── errors.go     → Erros de domínio
└── platform/
    ├── antifraude/   → Rate limiting com Redis
    ├── storage/      → Implementações de repositórios
    │   ├── postgres/ → Persistência (GORM)
    │   └── redis/    → Fila e contadores
    ├── migrations/   → Migrations automáticas (gormigrate)
    ├── logger/       → Logs estruturados (JSON)
    ├── metrics/      → Métricas Prometheus
    └── config/       → Configuração via env vars
```

---

## Stack Técnica

### Backend

- **Linguagem:** Go 1.25.1
- **Framework HTTP:** `net/http` padrão com `http.ServeMux` (sem router externo)
- **ORM:** GORM v2 (`gorm.io/gorm` v1.31.0)
- **Driver PostgreSQL:** `gorm.io/driver/postgres` v1.6.0 (usa `pgx/v5` internamente)
- **Client Redis:** `github.com/redis/go-redis/v9` v9.16.0
- **Migrations:** `github.com/go-gormigrate/gormigrate/v2` v2.1.5
- **ID Generator:** ULID via `github.com/oklog/ulid/v2` v2.1.1
- **Métricas:** `github.com/prometheus/client_golang` v1.19.0

### Infraestrutura

- **Banco de Dados:** PostgreSQL 16 (via Docker)
- **Cache/Fila:** Redis 7 (via Docker)
- **Containerização:** Docker multi-stage build + Docker Compose
- **Base Image:** `gcr.io/distroless/base-debian12` (produção)
- **Build Image:** `golang:1.25` (compilação)
- **Orquestração:** Kubernetes com Kind (cluster local)
- **Helm Charts:** Bitnami PostgreSQL + Redis (deploy K8s)

### Frontend

- **Renderização:** Server-Side Rendering (SSR) com `html/template` (stdlib)
- **Embed Templates:** `//go:embed` diretiva nativa do Go 1.16+
- **Localização:** `internal/app/web/templates/*.gohtml`
- **Estilo:** CSS inline (sem frameworks externos)
- **JavaScript:** Mínimo necessário (ex: auto-refresh no panorama)
- **Sem build step:** Templates compilados diretamente no binário

### Observabilidade

- **Logs:** JSON estruturado via `log/slog` (stdlib Go 1.21+)
- **Métricas:** Prometheus (`/metrics` na API + Worker)
- **Health Checks:** 
  - `/healthz` - Liveness probe (sempre retorna 200)
  - `/readyz` - Readiness probe (verifica Postgres + Redis)

### CI/CD

- **Automação:** Makefile + GitHub Actions
- **Testes de Carga:** k6 (`grafana/k6` via Docker)
- **Testes Unitários:** `go test` (stdlib)
- **Mock Redis:** `github.com/alicebob/miniredis/v2` v2.35.0 (testes)
- **Deploy Local:** Docker Compose
- **Deploy Kubernetes:** Kind + manifests YAML manuais

---

## Padrões e Convenções

### Estrutura de Código Go

1. **Nomenclatura:**
   - Packages: lowercase, singular (`voting`, `storage`)
   - Interfaces: sufixo `-Repository`, `-Service` quando aplicável
   - Structs: PascalCase
   - Métodos públicos: PascalCase
   - Métodos privados: camelCase

2. **Organização de Arquivos:**
   - Um conceito por arquivo (ex: `voto_repository.go`, `paredao_repository.go`)
   - Testes no mesmo diretório com sufixo `_test.go`
   - Mocks em subdiretório `mocks/` quando necessário

3. **Gestão de Erros:**
   - Use erros personalizados em `internal/domain/errors.go`
   - Sempre adicione contexto ao propagar erros: `fmt.Errorf("falha ao X: %w", err)`
   - Logs de erro devem incluir campos relevantes

4. **Context Propagation:**
   - Sempre passe `context.Context` como primeiro parâmetro
   - Use `ctx` para timeouts e cancelamentos
   - Propague ctx em todas as chamadas I/O

5. **HTTP Routing:**
   - Use `http.ServeMux` (stdlib) para rotas
   - Pattern matching manual com `strings.TrimPrefix` e `strings.Split`
   - Não há chi router ou gorilla/mux - é stdlib puro

### Templates Go (Frontend)

1. **Localização:** `internal/app/web/templates/*.gohtml`
2. **Páginas principais:**
   - `layout.gohtml` - Template base com CSS global
   - `vote.gohtml` - Interface de votação
   - `panorama.gohtml` - Resultados em tempo real
   - `consulta.gohtml` - Painel administrativo

3. **Paleta de Cores (BBB):**
   ```css
   /* Cores oficiais do BBB */
   --bbb-roxo: #5001b3;                /* Cor principal do BBB (roxo vibrante) */
   --bbb-roxo-escuro: #3d0189;         /* Roxo mais escuro para hover */
   --bbb-roxo-claro: #6c1ed9;          /* Roxo mais claro para gradientes */
   --bbb-rosa: #d7008d;                /* Rosa accent BBB */
   --bbb-branco: #ffffff;              /* Fundos e texto em gradientes */
   --bbb-cinza-claro: #f8f8f8;         /* Background geral */
   --bbb-cinza-medio: #e0e0e0;         /* Elementos neutros */
   --bbb-cinza-borda: #d0d0d0;         /* Bordas */
   --bbb-texto-escuro: #333333;        /* Texto principal */
   --bbb-texto-medio: #666666;         /* Texto secundário */
   --bbb-texto-claro: #999999;         /* Texto terciário */
   
   /* Gradientes característicos */
   background: linear-gradient(135deg, var(--bbb-roxo) 0%, var(--bbb-roxo-claro) 100%);
   /* Gradiente com rosa accent */
   background: linear-gradient(90deg, var(--bbb-roxo) 0%, var(--bbb-rosa) 50%, var(--bbb-roxo) 100%);
   ```

4. **Diretrizes de Estilo:**
   - Use CSS inline (não adicionar frameworks via CDN)
   - Mantenha consistência com paleta BBB (roxo #5001b3 como cor principal)
   - Evite JavaScript complexo (manter SSR simples)
   - Auto-refresh onde necessário (ex: panorama a cada 5s)
   - Use gradientes roxo→roxo-claro em headers, botões e elementos de destaque
   - Rosa (#d946ef) como accent color em detalhes e bordas animadas
   - Texto em caixa alta (uppercase) para títulos e labels
   - Sombras roxas para dar destaque visual (rgba(80, 1, 179, 0.x))

### Banco de Dados

1. **Migrations:**
   - Localização: `internal/platform/migrations/migrations.go`
   - Framework: `gormigrate` (versionamento automático)
   - Executadas automaticamente na inicialização (se `AUTO_MIGRATE=true`)
   - Seed de demonstração incluído (paredão com 3 participantes)
   - Nunca edite migrations já aplicadas em produção

2. **Modelos GORM:**
   - Tags obrigatórias: `gorm` e `json`
   - Use `gorm.Model` quando apropriado (ID, CreatedAt, UpdatedAt, DeletedAt)
   - Relações: definir FKs explicitamente

3. **Queries:**
   - Prefira métodos GORM sobre raw SQL
   - Use transações para operações múltiplas
   - Sempre trate erros de `gorm.ErrRecordNotFound`

---

## Restrições Importantes

### ❌ O Que NÃO Fazer

1. **Frameworks CSS Externos:**
   - ❌ NÃO adicionar Tailwind, Bootstrap via CDN
   - **Motivo:** Problemas com Content Security Policy (CSP)
   - **Alternativa:** CSS inline nos templates

2. **Mudar Arquitetura SSR:**
   - ❌ NÃO converter para SPA (React, Vue, etc.)
   - **Motivo:** Simplicidade é requisito, SSR é suficiente
   - **Exceção:** Apenas se houver justificativa forte

3. **Remover Rate Limiting:**
   - ❌ NÃO desabilitar antifraude em produção
   - **Motivo:** Requisito de negócio crítico
   - **Para testes:** Use `ANTIFRAUDE_RATE_LIMIT_ENABLED=false`

4. **Comprometer Performance:**
   - ❌ NÃO adicionar features que degradem o baseline de 1000 req/s
   - **Validação:** Sempre rodar `make perf-test` após mudanças

5. **Modificar Migrations Existentes:**
   - ❌ NÃO edite arquivos em `migrations/` já aplicados
   - **Alternativa:** Crie nova migration para correções

### ✅ Boas Práticas Obrigatórias

1. **Sempre documente decisões** em `COMMENTS.md`
2. **Rode testes após mudanças:** `go test ./...`
3. **Valide build Docker:** `make docker-rebuild`
4. **Teste carga crítica:** `make perf-test` (mudanças em API/Worker)
5. **Logs estruturados:** Use `logger.Info()`, `.Error()` com campos contextuais

---

## Fluxo de Desenvolvimento

### Desenvolvimento Local

```bash
# 1. Setup inicial
cp .env.example .env
make docker-up

# 2. Acompanhar logs
make logs        # API
make logs-worker # Worker

# 3. Após mudanças no código
make docker-rebuild

# 4. Testes
go test ./...           # Unitários
make perf-test         # Carga (k6)

# 5. Cleanup
make docker-down       # Parar containers
make docker-clean      # Remover volumes
```

### Fluxo de Trabalho Típico

1. **Entender requisito** → Consultar `docs/desafio.md`, `COMMENTS.md`
2. **Implementar mudança** → Seguir padrões deste documento
3. **Testar localmente** → `go test`, `make docker-rebuild`
4. **Validar performance** → `make perf-test` (se aplicável)
5. **Documentar decisão** → Atualizar `COMMENTS.md`
6. **Commit** → Mensagens claras e descritivas

---

## Comandos Essenciais

### Makefile Targets

```bash
# Docker Compose
make docker-up          # Subir stack completo (API + Worker + Postgres + Redis)
make docker-rebuild     # Rebuild e restart com mudanças
make docker-down        # Parar containers
make docker-clean       # Remover containers, volumes e redes
make logs               # Logs da API
make logs-worker        # Logs do Worker

# Testes
make perf-test          # Teste de carga com k6 (1000 req/s por 30s)

# Kubernetes
make deploy-kind        # Deploy completo em cluster Kind
make kind-delete        # Remover cluster Kind
```

### Comandos Docker Úteis

```bash
# Acessar banco de dados
docker compose exec postgres psql -U bbb -d bbb_votes

# Acessar Redis CLI
docker compose exec redis redis-cli

# Ver logs em tempo real
docker compose logs -f api worker

# Reiniciar apenas a API
docker compose restart api
```

### Testes e Validação

```bash
# Testes unitários
go test ./...
go test -v ./internal/app/voting/
go test -race ./...                    # Detectar race conditions
go test -cover ./...                   # Cobertura de testes

# Health checks
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics

# Testar votação manualmente
curl -X POST http://localhost:8080/votos \
  -H 'Content-Type: application/json' \
  -d '{"paredao_id":"<ID>","participante_id":"<ID>"}'
```

---

## Áreas Sensíveis

### 🔴 Modificações Críticas (Atenção Redobrada)

1. **Antifraude (`internal/platform/antifraude/`):**
   - Rate limiting baseado em Redis
   - Chave: `ratelimit:<IP>:<UserAgent>`
   - Limite padrão: 30 votos/min
   - **Impacto:** Segurança do sistema

2. **Worker (`internal/app/worker/`):**
   - Consome fila `votos:queue` do Redis
   - Processa votos assíncronamente
   - Atualiza contadores Redis + persiste Postgres
   - **Impacto:** Perda de votos se houver bugs

3. **Migrations (`internal/platform/migrations/`):**
   - Executadas automaticamente na inicialização
   - Usa `gormigrate` para versionamento
   - Cria schema e seed de demonstração
   - **Impacto:** Inconsistência de banco em produção

4. **Contadores Redis (`internal/platform/storage/redis/contador.go`):**
   - Mantém totais em tempo real com `INCR`
   - Chaves: `contador:paredao:<ID>`, `contador:participante:<ID>`
   - **Impacto:** Divergência entre Redis e Postgres

5. **Templates Web (`internal/app/web/templates/`):**
   - CSS inline com paleta Globo
   - Evitar sobrescrever estilos inline (precedência sobre CSS global)
   - **Impacto:** Inconsistência visual

### ⚠️ Cuidados Especiais

- **Nunca** commitar credenciais em `.env` (use `.env.example`)
- **Sempre** propague `context.Context` em operações I/O
- **Valide** rate limiting após mudanças em antifraude
- **Teste** worker após mudanças em fila/processamento
- **Confira** métricas Prometheus após deploys

---

## Diretrizes de Código

### Go Style Guide

1. **Siga:** [Effective Go](https://go.dev/doc/effective_go)
2. **Formatação:** Use `gofmt` (automático no save)
3. **Linting:** Projeto usa `golangci-lint` no CI
4. **Imports:**

   ```go
   import (
       // Standard library
       "context"
       "fmt"
       
       // External packages
       "github.com/prometheus/client_golang/prometheus/promhttp"
       
       // Internal packages
       "github.com/marcelojr/desafio-globo/internal/domain"
   )
   ```

### Tratamento de Erros

```go
// ✅ BOM: Erro com contexto
func (s *Service) ProcessarVoto(ctx context.Context, voto domain.Voto) error {
    if err := s.repo.Salvar(ctx, voto); err != nil {
        return fmt.Errorf("falha ao salvar voto: %w", err)
    }
    return nil
}

// ❌ RUIM: Erro sem contexto
func (s *Service) ProcessarVoto(ctx context.Context, voto domain.Voto) error {
    if err := s.repo.Salvar(ctx, voto); err != nil {
        return err
    }
    return nil
}
```

### Logging

```go
// ✅ BOM: Log estruturado com contexto via slog
logger.Info("votos processados com sucesso",
    "paredao_id", paredaoID,
    "total_votos", total)

logger.Error("falha ao processar voto",
    "voto_id", votoID,
    "err", err)

// ❌ RUIM: Log sem estrutura
fmt.Println("Votos processados:", total)
```

### Testes

```go
// Nomenclatura: Test<FuncaoOuMetodo>_<Cenario>
func TestProcessarVoto_QuandoValido_DeveRetornarSucesso(t *testing.T) {
    // Arrange
    service := NewService(mockRepo)
    voto := domain.Voto{/* ... */}
    
    // Act
    err := service.ProcessarVoto(context.Background(), voto)
    
    // Assert
    assert.NoError(t, err)
}
```

---

## Recursos Adicionais

### Documentação do Projeto

- **Desafio:** `docs/desafio.md` - Requisitos originais
- **Plano:** `docs/plano-execucao.md` - Estratégia de implementação
- **Testes:** `docs/roteiro-testes.md` - Guia de testes manuais
- **Decisões:** `COMMENTS.md` - Histórico de decisões técnicas

### Endpoints da API

```plaintext
GET  /vote                      # Interface de votação
GET  /panorama?paredao_id=<ID>  # Resultados em tempo real
GET  /consulta                  # Painel administrativo (requer token)
POST /votos                     # Registrar voto (JSON)
GET  /healthz                   # Liveness probe
GET  /readyz                    # Readiness probe (verifica Postgres + Redis)
GET  /metrics                   # Métricas Prometheus
```

### Variáveis de Ambiente Importantes

```bash
# Database
DATABASE_URL="postgres://bbb:bbb@localhost:5432/bbb_votes?sslmode=disable"

# Redis
REDIS_ADDR="localhost:6379"

# Servidor
HTTP_ADDR=":8080"

# Antifraude
ANTIFRAUDE_RATE_LIMIT_ENABLED=true
ANTIFRAUDE_RATE_LIMIT_MAX=30       # votos por janela
ANTIFRAUDE_RATE_LIMIT_WINDOW=60s   # janela de tempo

# Consulta (token de acesso)
CONSULTA_TOKEN="token-secreto-producao"

# Worker
WORKER_METRICS_ADDRESS=":9090"
```

---

## Orientações Finais para Agentes

### Antes de Fazer Qualquer Mudança

1. ✅ **Leia este arquivo completamente**
2. ✅ **Consulte `COMMENTS.md`** para decisões de negócio
3. ✅ **Verifique arquivos relacionados** à mudança proposta
4. ✅ **Entenda o impacto** nas áreas sensíveis
5. ✅ **Planeje testes** para validar a mudança

### Ao Sugerir Código

1. ✅ **Siga os padrões** deste documento
2. ✅ **Inclua tratamento de erros** adequado
3. ✅ **Adicione logs estruturados** onde relevante
4. ✅ **Proponha testes** para o código sugerido
5. ✅ **Documente a decisão** (atualizar `COMMENTS.md` se necessário)

### Ao Encontrar Algo Não Documentado

1. ✅ **Pergunte ao usuário** antes de assumir
2. ✅ **Sugira documentar** a decisão
3. ✅ **Mantenha consistência** com código existente

### Lembre-se

- 🎯 **Performance é prioridade:** 1000 req/s é o baseline
- 🔒 **Segurança é crítica:** Antifraude não é opcional
- 📝 **Documentação é obrigatória:** Decisões devem ser rastreáveis
- 🧪 **Testes são necessários:** Código sem teste é código frágil
- 🎨 **Paleta Globo é padrão:** Mantenha a identidade visual

---

**Última atualização:** 2025-11-01  
**Mantido por:** Time de Desenvolvimento

**Dúvidas?** Consulte `README.md`, `COMMENTS.md` ou pergunte ao usuário.
