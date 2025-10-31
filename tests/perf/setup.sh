#!/bin/bash
set -euo pipefail

API_BASE=${API_BASE:-http://localhost:${HTTP_PORT:-8080}}
PAREDAO_NAME="Perf Paredao"

# aguarda API subir
until curl -sS ${API_BASE}/healthz >/dev/null; do
  echo "Aguardando API..."
  sleep 1
done

# cria paredao
payload=$(
  cat <<JSON
{
  "nome": "$PAREDAO_NAME",
  "descricao": "teste",
  "inicio": "2025-10-30T20:00:00Z",
  "fim": "2025-11-02T20:00:00Z",
  "participantes": [
    {"nome": "Alice"},
    {"nome": "Bruno"}
  ]
}
JSON
)

RESPONSE=$(curl -sS -X POST ${API_BASE}/paredoes -H 'Content-Type: application/json' -d "$payload")

PAREDAO_ID=$(echo "$RESPONSE" | jq -r '.ID // .id')
PARTICIPANTE_IDS=$(echo "$RESPONSE" | jq -r '.Participantes[].ID // .participantes[].id' | paste -sd, -)

if [[ -z "$PAREDAO_ID" || -z "$PARTICIPANTE_IDS" ]]; then
  echo "Falha ao preparar dados: $RESPONSE"
  exit 1
fi

echo "PAREDAO_ID=$PAREDAO_ID" > tests/perf/runtime.env
echo "PARTICIPANTE_IDS=$PARTICIPANTE_IDS" >> tests/perf/runtime.env
