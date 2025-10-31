-- Seed inicial para paredão de demonstração
INSERT INTO paredoes (id, nome, descricao, inicio, fim, ativo, criado_em, atualizado_em)
VALUES ('demo_paredao_id', 'Paredão de demonstração', 'Seed inicial para testes do frontend', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '72 hour', TRUE, NOW(), NOW());

INSERT INTO participantes (id, paredao_id, nome, foto_url, criado_em, atualizado_em)
VALUES
  ('demo_participante_1', 'demo_paredao_id', 'Alice', NULL, NOW(), NOW()),
  ('demo_participante_2', 'demo_paredao_id', 'Bruno', NULL, NOW(), NOW()),
  ('demo_participante_3', 'demo_paredao_id', 'Carla', NULL, NOW(), NOW());
