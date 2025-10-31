// Pacote migrations centraliza as versões gormigrate aplicadas na inicialização.
package migrations

import (
	"fmt"

	gormigrate "github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
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
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("migrations: falha ao aplicar: %w", err)
	}

	return nil
}
