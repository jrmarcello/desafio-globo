# Plano de execução do desafio BBB

## Visão geral

Construir uma solução em Go que simule a votação de paredão do BBB, entregue uma API performática para registrar votos, exiba um painel web simples com o comprovante e a parcial de votos, e disponibilize métricas para acompanhar o total geral, por participante e por hora. Todo o trabalho precisa ser rastreável em um `COMMENTS.md` ou `HISTORY.md` na raiz, registrando decisões, testes realizados e ideias futuras.

## Requisitos funcionais

- Criar e gerenciar paredões com dois ou mais participantes.
- Registrar votos sem autenticação, permitindo múltiplos votos por usuário.
- Exibir ao final de cada voto a confirmação e a parcial percentual por participante.
- API para consultar totais (geral, por participante, por hora) de cada paredão.
- Interface web mínima chamando a API para enviar votos e mostrar a parcial.

## Requisitos não funcionais

- Manter a aplicação estável em picos de 1000 votos/seg.
- Evitar votos automatizados com uma camada antirrobô (reCAPTCHA ou hCaptcha) e limites por IP/UA.
- Disponibilizar logs estruturados, métricas e health checks para operação.
- Automatizar build, testes e deploy (Docker + docker-compose, Makefile e pipeline CI).

## Arquitetura proposta

- **API em Go** (net/http com router leve, ex. chi) expondo endpoints REST.
- **Serviço de contagem** desacoplado da camada HTTP; uso de Redis para contadores em tempo real e persistência final em PostgreSQL.
- **Consumidor assíncrono** que drena uma fila (Redis Stream ou canal interno) e grava no banco com idempotência.
- **Painel web** estático (HTML/JS) servido pela mesma API ou via S3/CloudFront, consumindo endpoints públicos.
- Instrumentação com Prometheus (metrics endpoint) e logs em JSON para fácil coleta.

## Etapas de implementação

1. **Fundação do projeto**: estrutura Go (cmd/internal), módulos, Makefile, Dockerfile, docker-compose com Postgres e Redis.
2. **Modelagem**: definir entidades (Paredao, Participante, Voto, ParcialHoraria) e migrar o banco (goose ou migrate).
3. **Serviços de domínio**: criar camada de aplicação com interfaces para voto, consulta e criação de paredões.
4. **Persistência**: implementar repositórios PostgreSQL e cache/contadores Redis.
5. **Fila e processamento**: endpoint de voto publica no Redis, worker aplica validações e atualiza banco e contadores.
6. **Endpoints REST**: criar handlers para:
   - criar/listar paredões e participantes,
   - registrar votos,
   - consultar totais, totas percentuais e séries por hora.
7. **Interface web**: páginas renderizadas com templates Go (SSR simples) servidas pela própria API: rota de votação, panorama pós-voto e painel de consulta; sem build step e focadas em consumir os endpoints existentes.
8. **Antifraude**: rate limiting por IP, verificação CAPTCHA e auditoria básica (logs e bloqueio temporário).
9. **Métricas e observabilidade**: expor `/metrics`, `/healthz` e dashboards básicos (configuração Prometheus/Grafana opcional documentada).
10. **Testes**: unitários para serviços/handlers, integração com banco/redis usando docker-compose, carga com k6 simulando 1000 req/s.
11. **Documentação**: preencher README com passos de setup, uso da API e da UI, e manter `COMMENTS.md` com anotações.
12. **Pipeline CI/CD**: workflow executando lint, testes e build; publicação de imagem Docker pronta para deploy containerizado.

## Estratégia de testes

- Unitários: lógica de contagem, validação antifraude, formatação de respostas.
- Integração: fluxo completo de voto ao persistir e retornar parciais.
- Contrato/API: validar status codes e payloads com httptest.
- Performance: script k6 executado via Makefile garantindo 1000 req/s com SLA definido (latência p95 < 200ms).
- Smoke pós-deploy: script simples que cria paredão, registra votos e verifica parciais.

## Deploy e operação

- Build da imagem Docker e publicação em registry privado/público.
- Deploy guiado via docker-compose (ambiente simples) e blueprint para Kubernetes (deployment + service + HPA).
- Variáveis de ambiente centralizadas (.env.example) para configuração (DB, Redis, CAPTCHA keys).
- Backups automáticos do banco e estratégia de rotação de logs.
