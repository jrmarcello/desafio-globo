// Pacote migrations centraliza as versões gormigrate aplicadas na inicialização.
package migrations

import (
	"fmt"
	"time"

	gormigrate "github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

func Run(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrations: db nulo")
	}

	// Usamos gormigrate para versionar as migrations sem depender de AutoMigrate direto em produção.
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		{
			ID: "202410310001_init_schema",
			Migrate: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&domain.Paredao{}, &domain.Participante{}, &domain.Voto{})
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("votos", "participantes", "paredoes")
			},
		},
		{
			ID: "202410310002_seed_demo_paredao",
			Migrate: func(tx *gorm.DB) error {
				var count int64
				if err := tx.Model(&domain.Paredao{}).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return nil
				}

				now := time.Now().Local()
				gen := ids.NewGenerator()
				paredaoID := domain.ParedaoID(gen.New())
				participantes := []domain.Participante{
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: "Alice Melo", CriadoEm: now, AtualizadoEm: now},
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: "Bruno Silva", CriadoEm: now, AtualizadoEm: now},
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: "Carla Souza", CriadoEm: now, AtualizadoEm: now},
				}

				seed := domain.Paredao{
					ID:        paredaoID,
					Nome:      "Paredão BBB",
					Descricao: "Paredão padrão para setup inicial",

					Inicio:        now.Add(-1 * time.Hour),
					Fim:           now.Add(72 * time.Hour),
					Participantes: participantes,
					Ativo:         true,
					CriadoEm:      now,
					AtualizadoEm:  now,
				}

				return tx.Create(&seed).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil
			},
		},
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("migrations: falha ao aplicar: %w", err)
	}

	return nil
}
