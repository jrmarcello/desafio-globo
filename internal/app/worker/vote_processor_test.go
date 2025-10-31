package worker

import (
	"context"
	"testing"
	"time"

	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/domain"
)

func TestVoteProcessorProcess(t *testing.T) {
	repo := &memVotoRepo{}
	contador := &memContador{valores: make(map[string]int64)}
	clock := &fixedClock{now: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)}

	processor := NewVoteProcessor(repo, contador, clock)

	voto := domain.Voto{
		ID:             "voto-1",
		ParedaoID:      "paredao-1",
		ParticipanteID: "participante-1",
	}

	if err := processor.Process(context.Background(), voto); err != nil {
		t.Fatalf("Process retornou erro inesperado: %v", err)
	}

	if len(repo.votos) != 1 {
		t.Fatalf("esperava 1 voto persistido, obteve %d", len(repo.votos))
	}
	if repo.votos[0].CriadoEm.IsZero() {
		t.Fatal("worker deveria preencher CriadoEm quando vazio")
	}

	total, ok := contador.valores[voting.CounterKeyTotalParedao(voto.ParedaoID)]
	if !ok || total != 1 {
		t.Fatalf("contador total deveria ser 1, veio %d (ok=%v)", total, ok)
	}

	partKey := voting.CounterKeyParticipante(voto.ParedaoID, voto.ParticipanteID)
	if contador.valores[partKey] != 1 {
		t.Fatalf("contador por participante deveria ser 1, veio %d", contador.valores[partKey])
	}
}

type memVotoRepo struct {
	votos []domain.Voto
}

func (m *memVotoRepo) Registrar(_ context.Context, voto domain.Voto) error {
	m.votos = append(m.votos, voto)
	return nil
}

func (m *memVotoRepo) TotalPorParedao(context.Context, domain.ParedaoID) (int64, error) {
	return 0, nil
}

func (m *memVotoRepo) TotalPorParticipante(context.Context, domain.ParedaoID) (map[domain.ParticipanteID]int64, error) {
	return nil, nil
}

func (m *memVotoRepo) TotalPorHora(context.Context, domain.ParedaoID) ([]domain.ParcialHora, error) {
	return nil, nil
}

type memContador struct {
	valores map[string]int64
}

func (m *memContador) Incrementar(_ context.Context, chave string, delta int64) (int64, error) {
	m.valores[chave] += delta
	return m.valores[chave], nil
}

func (m *memContador) Obter(_ context.Context, chave string) (int64, error) {
	return m.valores[chave], nil
}

func (m *memContador) ObterTodos(_ context.Context, chaves []string) (map[string]int64, error) {
	resultado := make(map[string]int64, len(chaves))
	for _, chave := range chaves {
		resultado[chave] = m.valores[chave]
	}
	return resultado, nil
}

type fixedClock struct {
	now time.Time
}

func (f *fixedClock) Agora() time.Time {
	return f.now
}
