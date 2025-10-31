// Pacote worker contém a lógica de processamento assíncrono dos votos provenientes da fila Redis.
package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/metrics"
)

// VoteProcessor grava votos no repositório e mantém contadores/ métricas.
type VoteProcessor struct {
	repo     domain.VotoRepository
	contador domain.Contador
	clock    domain.Clock
}

func NewVoteProcessor(repo domain.VotoRepository, contador domain.Contador, clock domain.Clock) *VoteProcessor {
	return &VoteProcessor{
		repo:     repo,
		contador: contador,
		clock:    clock,
	}
}

func (p *VoteProcessor) Process(ctx context.Context, voto domain.Voto) error {
	start := time.Now()

	// Se o voto veio da fila sem carimbo de data, usamos o clock do worker para registrar a chegada.
	if voto.CriadoEm.IsZero() {
		voto.CriadoEm = p.clock.Agora()
	}

	if err := p.repo.Registrar(ctx, voto); err != nil {
		return fmt.Errorf("worker: registrar voto %s: %w", voto.ID, err)
	}

	if p.contador == nil {
		// Quando o contador não está configurado, mantemos as métricas para monitorar o throughput.
		metrics.IncVoteProcessed()
		metrics.ObserveProcessingDuration(time.Since(start).Seconds())
		return nil
	}

	if _, err := p.contador.Incrementar(ctx, voting.CounterKeyTotalParedao(voto.ParedaoID), 1); err != nil {
		return fmt.Errorf("worker: incrementar contador total %s: %w", voto.ParedaoID, err)
	}

	if _, err := p.contador.Incrementar(ctx, voting.CounterKeyParticipante(voto.ParedaoID, voto.ParticipanteID), 1); err != nil {
		return fmt.Errorf("worker: incrementar contador participante %s/%s: %w", voto.ParedaoID, voto.ParticipanteID, err)
	}

	metrics.IncVoteProcessed()
	metrics.ObserveProcessingDuration(time.Since(start).Seconds())

	return nil
}
