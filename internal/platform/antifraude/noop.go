package antifraude

import (
	"context"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// Noop representa uma estratégia de antifraude desabilitada.
type Noop struct{}

func NewNoop() Noop {
	return Noop{}
}

func (Noop) Validar(ctx context.Context, voto domain.Voto) error {
	// Implementação vazia usada quando o rate limit é desligado via config.
	return nil
}
