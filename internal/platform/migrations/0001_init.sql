-- Schema inicial para paredões BBB
CREATE TABLE IF NOT EXISTS paredoes (
    id CHAR(26) PRIMARY KEY,
    nome TEXT NOT NULL,
    descricao TEXT,
    inicio TIMESTAMPTZ NOT NULL,
    fim TIMESTAMPTZ NOT NULL,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS participantes (
    id CHAR(26) PRIMARY KEY,
    paredao_id CHAR(26) NOT NULL REFERENCES paredoes (id) ON DELETE CASCADE,
    nome TEXT NOT NULL,
    foto_url TEXT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    atualizado_em TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS votos (
    id CHAR(26) PRIMARY KEY,
    paredao_id CHAR(26) NOT NULL REFERENCES paredoes (id) ON DELETE CASCADE,
    participante_id CHAR(26) NOT NULL REFERENCES participantes (id) ON DELETE CASCADE,
    origem_ip INET,
    user_agent TEXT,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_votos_paredao ON votos (paredao_id);
CREATE INDEX IF NOT EXISTS idx_votos_participante ON votos (participante_id);
CREATE INDEX IF NOT EXISTS idx_votos_paredao_criado_em ON votos (paredao_id, criado_em);
