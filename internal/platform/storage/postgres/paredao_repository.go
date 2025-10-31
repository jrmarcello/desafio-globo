package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// ParedaoRepository mapeia o agregado de paredão para tabelas GORM.
type ParedaoRepository struct {
	db *gorm.DB
}

func NewParedaoRepository(db *gorm.DB) *ParedaoRepository {
	return &ParedaoRepository{db: db}
}

type paredaoModel struct {
	ID            string              `gorm:"column:id;primaryKey"`
	Nome          string              `gorm:"column:nome"`
	Descricao     string              `gorm:"column:descricao"`
	Inicio        time.Time           `gorm:"column:inicio"`
	Fim           time.Time           `gorm:"column:fim"`
	Ativo         bool                `gorm:"column:ativo"`
	CriadoEm      time.Time           `gorm:"column:criado_em"`
	AtualizadoEm  time.Time           `gorm:"column:atualizado_em"`
	Participantes []participanteModel `gorm:"foreignKey:ParedaoID;references:ID"`
}

func (paredaoModel) TableName() string {
	return "paredoes"
}

func (m paredaoModel) toDomain(includeParticipants bool) domain.Paredao {
	p := domain.Paredao{
		ID:           domain.ParedaoID(m.ID),
		Nome:         m.Nome,
		Descricao:    m.Descricao,
		Inicio:       m.Inicio,
		Fim:          m.Fim,
		Ativo:        m.Ativo,
		CriadoEm:     m.CriadoEm,
		AtualizadoEm: m.AtualizadoEm,
	}

	if includeParticipants {
		participantes := make([]domain.Participante, len(m.Participantes))
		for i, part := range m.Participantes {
			participantes[i] = part.toDomain()
		}
		p.Participantes = participantes
	}

	return p
}

func fromDomainParedao(p domain.Paredao) paredaoModel {
	model := paredaoModel{
		ID:           string(p.ID),
		Nome:         p.Nome,
		Descricao:    p.Descricao,
		Inicio:       p.Inicio,
		Fim:          p.Fim,
		Ativo:        p.Ativo,
		CriadoEm:     p.CriadoEm,
		AtualizadoEm: p.AtualizadoEm,
	}

	if len(p.Participantes) > 0 {
		model.Participantes = make([]participanteModel, len(p.Participantes))
		for i, part := range p.Participantes {
			model.Participantes[i] = fromDomainParticipante(part)
		}
	}

	return model
}

func (r *ParedaoRepository) Create(ctx context.Context, p domain.Paredao) error {
	model := fromDomainParedao(p)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("gorm paredao: inserir: %w", err)
	}
	return nil
}

func (r *ParedaoRepository) Update(ctx context.Context, p domain.Paredao) error {
	model := fromDomainParedao(p)
	if err := r.db.WithContext(ctx).Model(&paredaoModel{}).
		Where("id = ?", model.ID).
		Updates(map[string]any{
			"nome":          model.Nome,
			"descricao":     model.Descricao,
			"inicio":        model.Inicio,
			"fim":           model.Fim,
			"ativo":         model.Ativo,
			"atualizado_em": model.AtualizadoEm,
		}).Error; err != nil {
		return fmt.Errorf("gorm paredao: atualizar: %w", err)
	}
	return nil
}

func (r *ParedaoRepository) FindByID(ctx context.Context, id domain.ParedaoID) (domain.Paredao, error) {
	var model paredaoModel
	if err := r.db.WithContext(ctx).
		// Preload garante relação pronta para ser convertida para o domínio.
		Preload("Participantes").
		First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Paredao{}, domain.ErrNotFound
		}
		return domain.Paredao{}, fmt.Errorf("gorm paredao: buscar id: %w", err)
	}
	return model.toDomain(true), nil
}

func (r *ParedaoRepository) ListAtivos(ctx context.Context) ([]domain.Paredao, error) {
	var models []paredaoModel
	if err := r.db.WithContext(ctx).
		// Critério acompanha a mesma regra aplicada na camada de domínio.
		Where("ativo = ? AND inicio <= NOW() AND fim >= NOW()", true).
		Order("inicio ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("gorm paredao: listar ativos: %w", err)
	}

	result := make([]domain.Paredao, len(models))
	for i, model := range models {
		result[i] = model.toDomain(false)
	}
	return result, nil
}

var _ domain.ParedaoRepository = (*ParedaoRepository)(nil)
