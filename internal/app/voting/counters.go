package voting

import (
	"fmt"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

func CounterKeyTotalParedao(id domain.ParedaoID) string {
	return fmt.Sprintf("paredao:%s:total", id)
}

func CounterKeyParticipante(paredaoID domain.ParedaoID, participanteID domain.ParticipanteID) string {
	return fmt.Sprintf("paredao:%s:participante:%s", paredaoID, participanteID)
}
