# Roteiro de testes manuais

Roteiro para guiar testes manuais. Ele reúne as sequências que usei para validar o comportamento da API, do worker e do ambiente em Docker/Kubernetes.

## Docker Compose

1. Certifique-se de que não há containers antigos: `docker compose down -v`
2. Suba o stack (API, worker, Postgres, Redis): `docker compose up -d --build`
3. Verifique os logs da API para confirmar migrations e inicialização: `docker compose logs api`
4. (Opcional) Verifique logs do worker: `docker compose logs worker`
5. Cheque observabilidade:

   ```bash
   curl -sS http://localhost:${HTTP_PORT:-8080}/readyz
   curl -sS http://localhost:${HTTP_PORT:-8080}/metrics | head -n 20
   docker compose logs api | tail -n 20
   docker compose exec worker curl -sS http://localhost:9090/metrics | head -n 20
   ```

   - `/readyz` responde `ok` ao garantir Postgres/Redis disponíveis.
   - `/metrics` traz contadores Prometheus (ex.: `bbb_vote_requests_total`).
   - Os logs aparecem em JSON com campos como `level`, `msg`, `addr`.
   - O worker expõe `/metrics` em `:9090` (alterável via `WORKER_METRICS_ADDRESS`).

6. Faça uma requisição de criação de paredão:

   ```bash
   curl -sS -X POST http://localhost:${HTTP_PORT:-8080}/paredoes \
     -H 'Content-Type: application/json' \
     -d '{
           "nome": "Paredao Local",
           "descricao": "teste",
           "inicio": "2025-10-30T20:00:00Z",
           "fim": "2025-11-02T20:00:00Z",
           "participantes": [
             {"nome": "Alice"},
             {"nome": "Bruno"}
           ]
         }'
   ```

7. Registre um voto:

   ```bash
   curl -sS -X POST http://localhost:${HTTP_PORT:-8080}/votos \
     -H 'Content-Type: application/json' \
     -d '{
           "paredao_id": "<ID_do_paredao>",
           "participante_id": "<ID_do_participante>"
         }'
   ```

8. Valide o antifraude (limite padrão de 30 votos/min por IP+UA):

   ```bash
   for i in $(seq 1 35); do
     curl -s -o /dev/null -w '%{http_code}\n' \
       -X POST http://localhost:${HTTP_PORT:-8080}/votos \
       -H 'Content-Type: application/json' \
       -d '{
             "paredao_id": "<ID_do_paredao>",
             "participante_id": "<ID_do_participante>"
           }'
   done
   ```

   - As primeiras respostas retornam `202`; após o limite, a API passa a responder `429` (`rate_limited`).

9. Derrube tudo ao final: `docker compose down -v`

## Frontend (SSR)
- Após subir o stack (`docker compose up -d --build`), crie um paredão via `POST /paredoes` (ex.: usando `curlimages/curl` na rede `desafio-globo_default`).
- Acesse `/vote` para conferir o paredão ativo e os botões de voto.
- Submeter o formulário direciona para `/panorama?paredao_id=...`, onde o comprovante e as parciais são mostrados.
- A `/consulta` pede o token configurado em `CONSULTA_TOKEN`; após informar, o painel exibe totais por participante e por hora.

### Teste de carga (k6)

Requerimentos: Docker (Desktop), `jq` e acesso à imagem `grafana/k6`.

1. `make perf-test`
   - sobe o stack via docker-compose,
   - cria um paredão participante (`tests/perf/setup.sh` usa a API),
   - roda k6 com taxa padrão de 1000 req/s por 30s.
2. Ajuste as variáveis usando `RATE=1500 DURATION=60s make perf-test` se quiser outro cenário.
3. No final o comando derruba o stack e remove arquivos temporários.
4. Resultados (última execução): 30k votos aceitos, p95 ≈ 1.15ms, nenhum erro.

## Kubernetes com kind

> Atalho: `make deploy-kind` realiza automaticamente o ciclo completo (cria cluster, instala dependências, faz build/load das imagens, aplica manifestos e executa um smoke test com curl). Para destruir o ambiente depois, use `make kind-delete`.

1. Crie o cluster (1 control-plane + 2 workers):

   ```bash
   kind create cluster --name votacao-paredao-bbb --config deploy/k8s/kind-cluster.yaml
   ```

2. Namespace e dependências:

   ```bash
   kubectl apply -f deploy/k8s/namespace.yaml
   helm install postgres bitnami/postgresql -n votacao-paredao-bbb --set auth.username=bbb --set auth.password=bbb --set auth.database=bbb_votes
   helm install redis bitnami/redis -n votacao-paredao-bbb --set architecture=standalone --set auth.enabled=false
   ```

3. Carregue as imagens locais no kind:

   ```bash
   kind load docker-image votacao-paredao-bbb-api:latest --name votacao-paredao-bbb
   kind load docker-image votacao-paredao-bbb-worker:latest --name votacao-paredao-bbb
   ```

4. Aplique ConfigMap e Secret:

   ```bash
   kubectl apply -f deploy/k8s/configmap.yaml -f deploy/k8s/secret.yaml
   ```

5. Suba os deployments e service (2 réplicas cada):

   ```bash
   kubectl apply -f deploy/k8s/deployment-api.yaml -f deploy/k8s/deployment-worker.yaml -f deploy/k8s/service-api.yaml
   ```

6. Aguarde os pods ficarem prontos: `kubectl get pods -n votacao-paredao-bbb`

7. Verifique logs da API: `kubectl logs deployment/votacao-paredao-bbb-api -n votacao-paredao-bbb`

8. Observe a saúde da API:

   ```bash
   kubectl run curl --rm --restart=Never --namespace votacao-paredao-bbb \
     --image=curlimages/curl --command -- \
     curl -sS http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/readyz
   kubectl run curl --rm --restart=Never --namespace votacao-paredao-bbb \
     --image=curlimages/curl --command -- \
     curl -sS http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/metrics | head -n 20
   kubectl logs deployment/votacao-paredao-bbb-api -n votacao-paredao-bbb | tail -n 20

   ```text
   - `/readyz` confirma conexão com banco e cache dentro do cluster.
   - `/metrics` traz os contadores expostos para Prometheus.
   - Os logs em JSON ajudam a diagnosticar falhas rapidamente.
   - Para observar métricas do worker:

     ```bash
     WORKER_POD=$(kubectl get pods -n votacao-paredao-bbb -l app=votacao-paredao-bbb-worker -o jsonpath='{.items[0].metadata.name}')
     kubectl exec -n votacao-paredao-bbb "$WORKER_POD" -- curl -s http://localhost:9090/metrics | head -n 20
     ```

9. Crie um paredão dentro do cluster:

   ```bash
   kubectl run curl --rm -i --tty --restart=Never --namespace votacao-paredao-bbb \
     --image=curlimages/curl --command -- \
     curl -sS -X POST http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/paredoes \
     -H 'Content-Type: application/json' \
     -d '{
           "nome": "Paredao K8s",
           "descricao": "teste",
           "inicio": "2025-10-30T20:00:00Z",
           "fim": "2025-11-02T20:00:00Z",
           "participantes": [
             {"nome": "Alice"},
             {"nome": "Bruno"}
           ]
         }'
   ```

10. Registre um voto:

    ```bash
    kubectl run curl --rm -i --tty --restart=Never --namespace votacao-paredao-bbb \
      --image=curlimages/curl --command -- \
      curl -sS -o /dev/null -w '%{http_code}' -X POST \
      http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/votos \
      -H 'Content-Type: application/json' \
      -d '{
            "paredao_id": "<ID_do_paredao>",
            "participante_id": "<ID_do_participante>"
          }'
    ```

11. Valide o antifraude (mesmo IP/UA dentro do cluster):

    ```bash
    kubectl run curl --rm --restart=Never --namespace votacao-paredao-bbb \
      --image=curlimages/curl --command -- /bin/sh -c '
        for i in $(seq 1 35); do \
          curl -s -o /dev/null -w "%{http_code}\\n" \
            -X POST http://votacao-paredao-bbb-api.votacao-paredao-bbb.svc.cluster.local:8080/votos \
            -H "Content-Type: application/json" \
            -d "{\\\"paredao_id\\\":\\\"<ID_do_paredao>\\\",\\\"participante_id\\\":\\\"<ID_do_participante>\\\"}"; \
        done'
    ```

    - Observe respostas `202` no início e `429` quando o limite é atingido.

12. Limpe o cluster ao final:

    ```bash
    kind delete cluster --name votacao-paredao-bbb
    ```
