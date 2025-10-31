// Pacote voting implementa as regras de negócio do paredão: criação, votação e leitura de parciais.
package voting

import (
	"context"
	"errors"
	"fmt"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

var (
	ErrParedaoInvalido          = errors.New("paredao invalido")
	ErrPeriodoEncerrado         = errors.New("paredao encerrado")
	ErrParticipanteDesconhecido = errors.New("participante nao encontrado")
	ErrParedaoNaoEncontrado     = errors.New("paredao nao encontrado")
)

// Service concentra as regras de votação e delega acesso a repositórios/fila.
type Service struct {
	paredoes      domain.ParedaoRepository
	participantes domain.ParticipanteRepository
	votos         domain.VotoRepository
	contador      domain.Contador
	fila          domain.Fila
	antifraude    domain.Antifraude
	clock         domain.Clock
	ids           *ids.Generator
}

func NewService(
	paredoes domain.ParedaoRepository,
	participantes domain.ParticipanteRepository,
	votos domain.VotoRepository,
	contador domain.Contador,
	fila domain.Fila,
	antifraude domain.Antifraude,
	clock domain.Clock,
	idsGen *ids.Generator,
) *Service {
	if idsGen == nil {
		idsGen = ids.DefaultGenerator()
	}
	return &Service{
		paredoes:      paredoes,
		participantes: participantes,
		votos:         votos,
		contador:      contador,
		fila:          fila,
		antifraude:    antifraude,
		clock:         clock,
		ids:           idsGen,
	}
}

// CriarParedao centraliza a validação e a criação das entidades principais dentro de uma única transação lógica.
func (s *Service) CriarParedao(ctx context.Context, p domain.Paredao, participantes []domain.Participante) (domain.Paredao, error) {
	if err := validarParedao(p, participantes); err != nil {
		return domain.Paredao{}, err
	}
	agora := s.clock.Agora()

	p.ID = domain.ParedaoID(s.ids.New())
	if p.Inicio.IsZero() {
		p.Inicio = agora
	}
	if p.Fim.IsZero() || p.Fim.Before(p.Inicio) {
		return domain.Paredao{}, fmt.Errorf("%w: intervalo invalido", ErrParedaoInvalido)
	}
	p.Ativo = true
	p.CriadoEm = agora
	p.AtualizadoEm = agora

	participantesCriados := make([]domain.Participante, len(participantes))
	for i, part := range participantes {
		part.ID = domain.ParticipanteID(s.ids.New())
		part.ParedaoID = p.ID
		part.CriadoEm = agora
		part.AtualizadoEm = agora
		participantesCriados[i] = part
	}

	if err := s.paredoes.Create(ctx, p); err != nil {
		return domain.Paredao{}, err
	}

	if err := s.participantes.BulkCreate(ctx, p.ID, participantesCriados); err != nil {
		return domain.Paredao{}, err
	}

	p.Participantes = participantesCriados
	return p, nil
}

func (s *Service) ListarAtivos(ctx context.Context) ([]domain.Paredao, error) {
	paredoes, err := s.paredoes.ListAtivos(ctx)
	if err != nil {
		return nil, err
	}

	for i := range paredoes {
		participantes, pErr := s.participantes.ListByParedao(ctx, paredoes[i].ID)
		if pErr != nil {
			return nil, pErr
		}
		paredoes[i].Participantes = participantes
	}

	return paredoes, nil
}

// RegistrarVoto aplica as regras de negócio antes de delegar à fila (modo assíncrono) ou ao repositório.
func (s *Service) RegistrarVoto(ctx context.Context, voto domain.Voto) error {
	if voto.ParedaoID == "" || voto.ParticipanteID == "" {
		return ErrParticipanteDesconhecido
	}
	paredao, err := s.paredoes.FindByID(ctx, voto.ParedaoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrParedaoNaoEncontrado
		}
		return err
	}

	agora := s.clock.Agora()
	if !paredao.Ativo || agora.Before(paredao.Inicio) || agora.After(paredao.Fim) {
		return ErrPeriodoEncerrado
	}

	participantes, err := s.participantes.ListByParedao(ctx, voto.ParedaoID)
	if err != nil {
		return err
	}

	if !participanteExiste(participantes, voto.ParticipanteID) {
		return ErrParticipanteDesconhecido
	}

	if s.antifraude != nil {
		if err := s.antifraude.Validar(ctx, voto); err != nil {
			return err
		}
	}

	voto.ID = domain.VotoID(s.ids.New())
	voto.CriadoEm = agora

	if s.fila != nil {
		// No modo assíncrono basta publicar; o worker cuidará da persistência e contadores.
		return s.fila.PublicarVoto(ctx, voto)
	}

	if err := s.votos.Registrar(ctx, voto); err != nil {
		return err
	}

	if s.contador != nil {
		if _, err := s.contador.Incrementar(ctx, CounterKeyTotalParedao(voto.ParedaoID), 1); err != nil {
			return err
		}
		if _, err := s.contador.Incrementar(ctx, CounterKeyParticipante(voto.ParedaoID, voto.ParticipanteID), 1); err != nil {
			return err
		}
	}

	return nil
}

// Parciais lê contadores do Postgres para manter consistência mesmo sem Redis.
func (s *Service) Parciais(ctx context.Context, paredaoID domain.ParedaoID) ([]domain.Parcial, error) {
	_, err := s.paredoes.FindByID(ctx, paredaoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrParedaoNaoEncontrado
		}
		return nil, err
	}

	participantes, err := s.participantes.ListByParedao(ctx, paredaoID)
	if err != nil {
		return nil, err
	}

	totais, err := s.votos.TotalPorParticipante(ctx, paredaoID)
	if err != nil {
		return nil, err
	}

	var totalGeral int64
	for _, total := range totais {
		totalGeral += total
	}

	if totalGeral == 0 {
		resultado := make([]domain.Parcial, len(participantes))
		for i, part := range participantes {
			resultado[i] = domain.Parcial{
				ParedaoID:      paredaoID,
				ParticipanteID: part.ID,
				Total:          0,
				Percentual:     0,
			}
		}
		return resultado, nil
	}

	resultado := make([]domain.Parcial, len(participantes))
	for i, part := range participantes {
		total := totais[part.ID]
		percentual := (float64(total) / float64(totalGeral)) * 100
		resultado[i] = domain.Parcial{
			ParedaoID:      paredaoID,
			ParticipanteID: part.ID,
			Total:          total,
			Percentual:     percentual,
		}
	}

	return resultado, nil
}

func (s *Service) TotaisPorHora(ctx context.Context, paredaoID domain.ParedaoID) ([]domain.ParcialHora, error) {
	_, err := s.paredoes.FindByID(ctx, paredaoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrParedaoNaoEncontrado
		}
		return nil, err
	}
	return s.votos.TotalPorHora(ctx, paredaoID)
}

func validarParedao(p domain.Paredao, participantes []domain.Participante) error {
	if p.Nome == "" {
		return fmt.Errorf("%w: nome obrigatorio", ErrParedaoInvalido)
	}
	if len(participantes) < 2 {
		return fmt.Errorf("%w: minimo de dois participantes", ErrParedaoInvalido)
	}
	return nil
}

func participanteExiste(participantes []domain.Participante, id domain.ParticipanteID) bool {
	for _, part := range participantes {
		if part.ID == id {
			return true
		}
	}
	return false
}
