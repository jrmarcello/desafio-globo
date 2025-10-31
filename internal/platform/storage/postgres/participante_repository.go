package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// ParticipanteRepository persiste participantes associados a um paredão usando GORM.
type ParticipanteRepository struct {
	db *gorm.DB
}

func NewParticipanteRepository(db *gorm.DB) *ParticipanteRepository {
	return &ParticipanteRepository{db: db}
}

type participanteModel struct {
	ID           string    `gorm:"column:id;primaryKey"`
	ParedaoID    string    `gorm:"column:paredao_id;index"`
	Nome         string    `gorm:"column:nome"`
	FotoURL      string    `gorm:"column:foto_url"`
	CriadoEm     time.Time `gorm:"column:criado_em"`
	AtualizadoEm time.Time `gorm:"column:atualizado_em"`
}

func (participanteModel) TableName() string {
	return "participantes"
}

func (m participanteModel) toDomain() domain.Participante {
	return domain.Participante{
		ID:           domain.ParticipanteID(m.ID),
		ParedaoID:    domain.ParedaoID(m.ParedaoID),
		Nome:         m.Nome,
		FotoURL:      m.FotoURL,
		CriadoEm:     m.CriadoEm,
		AtualizadoEm: m.AtualizadoEm,
	}
}

func fromDomainParticipante(p domain.Participante) participanteModel {
	return participanteModel{
		ID:           string(p.ID),
		ParedaoID:    string(p.ParedaoID),
		Nome:         p.Nome,
		FotoURL:      p.FotoURL,
		CriadoEm:     p.CriadoEm,
		AtualizadoEm: p.AtualizadoEm,
	}
}

func (r *ParticipanteRepository) BulkCreate(ctx context.Context, paredaoID domain.ParedaoID, participantes []domain.Participante) error {
	if len(participantes) == 0 {
		return nil
	}

	// Popular o slice evita múltiplos round-trips: inserimos todos os participantes de uma vez.
	models := make([]participanteModel, len(participantes))
	for i, part := range participantes {
		if part.ParedaoID == "" {
			part.ParedaoID = paredaoID
		}
		models[i] = fromDomainParticipante(part)
	}

	if err := r.db.WithContext(ctx).Create(&models).Error; err != nil {
		return fmt.Errorf("gorm participante: bulk create: %w", err)
	}
	return nil
}

func (r *ParticipanteRepository) ListByParedao(ctx context.Context, paredaoID domain.ParedaoID) ([]domain.Participante, error) {
	var models []participanteModel
	if err := r.db.WithContext(ctx).
		// Ordenamos por nome para manter previsibilidade na API e em relatórios.
		Where("paredao_id = ?", paredaoID).
		Order("nome ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("gorm participante: listar: %w", err)
	}

	result := make([]domain.Participante, len(models))
	for i, model := range models {
		result[i] = model.toDomain()
	}
	return result, nil
}

var _ domain.ParticipanteRepository = (*ParticipanteRepository)(nil)
