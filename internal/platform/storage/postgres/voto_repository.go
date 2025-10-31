package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// VotoRepository guarda votos e expõe consultas agregadas próprias do Postgres.
type VotoRepository struct {
	db *gorm.DB
}

func NewVotoRepository(db *gorm.DB) *VotoRepository {
	return &VotoRepository{db: db}
}

type votoModel struct {
	ID             string    `gorm:"column:id;primaryKey"`
	ParedaoID      string    `gorm:"column:paredao_id;index"`
	ParticipanteID string    `gorm:"column:participante_id;index"`
	OrigemIP       string    `gorm:"column:origem_ip"`
	UserAgent      string    `gorm:"column:user_agent"`
	CriadoEm       time.Time `gorm:"column:criado_em"`
}

func (votoModel) TableName() string {
	return "votos"
}

func fromDomainVoto(v domain.Voto) votoModel {
	return votoModel{
		ID:             string(v.ID),
		ParedaoID:      string(v.ParedaoID),
		ParticipanteID: string(v.ParticipanteID),
		OrigemIP:       v.OrigemIP,
		UserAgent:      v.UserAgent,
		CriadoEm:       v.CriadoEm,
	}
}

func (r *VotoRepository) Registrar(ctx context.Context, voto domain.Voto) error {
	model := fromDomainVoto(voto)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("gorm votos: inserir: %w", err)
	}
	return nil
}

func (r *VotoRepository) TotalPorParedao(ctx context.Context, id domain.ParedaoID) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&votoModel{}).
		Where("paredao_id = ?", id).
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("gorm votos: total paredao: %w", err)
	}
	return total, nil
}

func (r *VotoRepository) TotalPorParticipante(ctx context.Context, paredaoID domain.ParedaoID) (map[domain.ParticipanteID]int64, error) {
	type resultado struct {
		ParticipanteID string
		Total          int64
	}
	var res []resultado
	if err := r.db.WithContext(ctx).
		Model(&votoModel{}).
		Select("participante_id as participante_id, COUNT(*) as total").
		Where("paredao_id = ?", paredaoID).
		Group("participante_id").
		Scan(&res).Error; err != nil {
		return nil, fmt.Errorf("gorm votos: total participante: %w", err)
	}

	totais := make(map[domain.ParticipanteID]int64, len(res))
	for _, item := range res {
		totais[domain.ParticipanteID(item.ParticipanteID)] = item.Total
	}
	return totais, nil
}

func (r *VotoRepository) TotalPorHora(ctx context.Context, paredaoID domain.ParedaoID) ([]domain.ParcialHora, error) {
	type resultado struct {
		Hora  time.Time
		Total int64
	}

	var res []resultado
	if err := r.db.WithContext(ctx).
		// Usamos SQL cru para aproveitar o `date_trunc` do Postgres sem montar lógica manual.
		Raw(`
            SELECT date_trunc('hour', criado_em) AS hora, COUNT(*) AS total
            FROM votos
            WHERE paredao_id = ?
            GROUP BY hora
            ORDER BY hora ASC
        `, paredaoID).
		Scan(&res).Error; err != nil {
		return nil, fmt.Errorf("gorm votos: total hora: %w", err)
	}

	parciais := make([]domain.ParcialHora, len(res))
	for i, item := range res {
		parciais[i] = domain.ParcialHora{
			ParedaoID: domain.ParedaoID(paredaoID),
			Hora:      item.Hora,
			Total:     item.Total,
		}
	}
	return parciais, nil
}

var _ domain.VotoRepository = (*VotoRepository)(nil)
