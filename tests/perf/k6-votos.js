import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  scenarios: {
    votos: {
      executor: 'constant-arrival-rate',
      rate: __ENV.RATE || 1000, // requisições por segundo
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: __ENV.PRE_VUS || 200,
      maxVUs: __ENV.MAX_VUS || 400,
    },
  },
};

const API_BASE = __ENV.API_BASE || 'http://localhost:8080';
const PAREDAO_ID = __ENV.PAREDAO_ID;
const PARTICIPANTE_IDS = (__ENV.PARTICIPANTE_IDS || '').split(',').filter(Boolean);

if (!PAREDAO_ID || PARTICIPANTE_IDS.length < 2) {
  throw new Error('Defina PAREDAO_ID e PARTICIPANTE_IDS (duas IDs separadas por vírgula) nas variáveis de ambiente');
}

export default function () {
  const participante = PARTICIPANTE_IDS[Math.floor(Math.random() * PARTICIPANTE_IDS.length)];
  const headers = {
    'Content-Type': 'application/json',
    'X-Forwarded-For': `198.18.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
    'User-Agent': `k6-perf/${__VU}`,
  };

  const res = http.post(`${API_BASE}/votos`, JSON.stringify({
    paredao_id: PAREDAO_ID,
    participante_id: participante,
  }), { headers });

  check(res, {
    'status 202 ou 200': (r) => r.status === 202 || r.status === 200,
  });

  sleep(0.001);
}
