// Pacote migrations centraliza as versões gormigrate aplicadas na inicialização.
package migrations

import (
	"fmt"
	"os"
	"strings"
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
				// Só cria seed se a tabela estiver vazia
				var count int64
				if err := tx.Model(&domain.Paredao{}).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return nil
				}

				// Cria paredão de demonstração com dados parametrizados via env vars
				now := time.Now().Local()
				gen := ids.NewGenerator()
				paredaoID := domain.ParedaoID(gen.New())

				// Lê nome do paredão do env (default: "Paredão BBB - Semana 1")
				paredaoNome := getEnv("SEED_PAREDAO_NOME", "Paredão BBB - Semana 1")

				// Lê nomes dos participantes do env
				participante1 := getEnv("SEED_PARTICIPANTE_1", "Alice Melo")
				participante2 := getEnv("SEED_PARTICIPANTE_2", "Bruno Silva")
				participante3 := getEnv("SEED_PARTICIPANTE_3", "Carla Souza")

				participantes := []domain.Participante{
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: participante1, CriadoEm: now, AtualizadoEm: now},
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: participante2, CriadoEm: now, AtualizadoEm: now},
					{ID: domain.ParticipanteID(gen.New()), ParedaoID: paredaoID, Nome: participante3, CriadoEm: now, AtualizadoEm: now},
				}

				seed := domain.Paredao{
					ID:            paredaoID,
					Nome:          paredaoNome,
					Descricao:     "Paredão de demonstração criado automaticamente",
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

// getEnv retorna variável de ambiente ou valor default
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}
