package domain

import (
	"context"
	"time"
)

type ParedaoRepository interface {
	Create(ctx context.Context, p Paredao) error
	Update(ctx context.Context, p Paredao) error
	FindByID(ctx context.Context, id ParedaoID) (Paredao, error)
	ListAtivos(ctx context.Context) ([]Paredao, error)
}

type ParticipanteRepository interface {
	BulkCreate(ctx context.Context, paredaoID ParedaoID, participantes []Participante) error
	ListByParedao(ctx context.Context, paredaoID ParedaoID) ([]Participante, error)
}

type VotoRepository interface {
	Registrar(ctx context.Context, voto Voto) error
	TotalPorParedao(ctx context.Context, id ParedaoID) (int64, error)
	TotalPorParticipante(ctx context.Context, paredaoID ParedaoID) (map[ParticipanteID]int64, error)
	TotalPorHora(ctx context.Context, paredaoID ParedaoID) ([]ParcialHora, error)
}

type Contador interface {
	Incrementar(ctx context.Context, chave string, delta int64) (int64, error)
	Obter(ctx context.Context, chave string) (int64, error)
	ObterTodos(ctx context.Context, chaves []string) (map[string]int64, error)
}

type Fila interface {
	PublicarVoto(ctx context.Context, voto Voto) error
	ConsumirVotos(ctx context.Context, handler func(context.Context, Voto) error) error
}

type Antifraude interface {
	Validar(ctx context.Context, voto Voto) error
}

type Clock interface {
	Agora() time.Time
}

type VotingService interface {
	RegistrarVoto(ctx context.Context, voto Voto) error
	ListarAtivos(ctx context.Context) ([]Paredao, error)
	Parciais(ctx context.Context, id ParedaoID) ([]Parcial, error)
	TotaisPorHora(ctx context.Context, id ParedaoID) ([]ParcialHora, error)
	CriarParedao(ctx context.Context, paredao Paredao, participantes []Participante) (Paredao, error)
}
