package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

func TestParticipanteRepository_BulkCreate_QuandoValidos_DevePersistirTodos(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange
	participantes := []domain.Participante{
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Alice Melo",
			FotoURL:   "https://example.com/alice.jpg",
			CriadoEm:  now,
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Bruno Silva",
			FotoURL:   "https://example.com/bruno.jpg",
			CriadoEm:  now,
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Carla Souza",
			FotoURL:   "",
			CriadoEm:  now,
		},
	}

	// Act
	err := repo.BulkCreate(ctx, paredaoID, participantes)
	require.NoError(t, err)

	// Assert
	listados, err := repo.ListByParedao(ctx, paredaoID)
	assert.NoError(t, err)
	assert.Len(t, listados, 3)

	// Verificar ordem alfabética
	assert.Equal(t, "Alice Melo", listados[0].Nome)
	assert.Equal(t, "Bruno Silva", listados[1].Nome)
	assert.Equal(t, "Carla Souza", listados[2].Nome)
}

func TestParticipanteRepository_BulkCreate_QuandoListaVazia_NaoDeveFazerNada(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())

	// Act
	err := repo.BulkCreate(ctx, paredaoID, []domain.Participante{})

	// Assert
	assert.NoError(t, err)

	// Verificar que não há participantes
	listados, err := repo.ListByParedao(ctx, paredaoID)
	assert.NoError(t, err)
	assert.Empty(t, listados)
}

func TestParticipanteRepository_BulkCreate_QuandoParedaoIDVazio_DeveUsarParametro(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Participante sem ParedaoID definido
	participante := domain.Participante{
		ID:       domain.ParticipanteID(gen.New()),
		Nome:     "Teste Participante",
		CriadoEm: now,
	}

	// Act
	err := repo.BulkCreate(ctx, paredaoID, []domain.Participante{participante})
	require.NoError(t, err)

	// Assert
	listados, err := repo.ListByParedao(ctx, paredaoID)
	assert.NoError(t, err)
	assert.Len(t, listados, 1)
	assert.Equal(t, paredaoID, listados[0].ParedaoID)
}

func TestParticipanteRepository_ListByParedao_QuandoExistemParticipantes_DeveRetornarOrdenados(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Criar participantes fora de ordem alfabética
	participantes := []domain.Participante{
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Zara Oliveira",
			CriadoEm:  now,
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Ana Costa",
			CriadoEm:  now,
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Maria Santos",
			CriadoEm:  now,
		},
	}

	err := repo.BulkCreate(ctx, paredaoID, participantes)
	require.NoError(t, err)

	// Act
	listados, err := repo.ListByParedao(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, listados, 3)

	// Deve estar ordenado alfabeticamente
	assert.Equal(t, "Ana Costa", listados[0].Nome)
	assert.Equal(t, "Maria Santos", listados[1].Nome)
	assert.Equal(t, "Zara Oliveira", listados[2].Nome)
}

func TestParticipanteRepository_ListByParedao_QuandoParedaoNaoExiste_DeveRetornarListaVazia(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoIDInexistente := domain.ParedaoID(gen.New())

	// Act
	listados, err := repo.ListByParedao(ctx, paredaoIDInexistente)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, listados)
}

func TestParticipanteRepository_ListByParedao_QuandoMultiplosParedoes_DeveRetornarApenasDoParedaoCorreto(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParticipanteRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID1 := domain.ParedaoID(gen.New())
	paredaoID2 := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Criar participantes para dois paredões diferentes
	participantes1 := []domain.Participante{
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID1,
			Nome:      "Participante P1-A",
			CriadoEm:  now,
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID1,
			Nome:      "Participante P1-B",
			CriadoEm:  now,
		},
	}

	participantes2 := []domain.Participante{
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID2,
			Nome:      "Participante P2-A",
			CriadoEm:  now,
		},
	}

	err := repo.BulkCreate(ctx, paredaoID1, participantes1)
	require.NoError(t, err)

	err = repo.BulkCreate(ctx, paredaoID2, participantes2)
	require.NoError(t, err)

	// Act
	listados1, err := repo.ListByParedao(ctx, paredaoID1)
	assert.NoError(t, err)

	listados2, err := repo.ListByParedao(ctx, paredaoID2)
	assert.NoError(t, err)

	// Assert
	assert.Len(t, listados1, 2)
	assert.Len(t, listados2, 1)

	assert.Equal(t, "Participante P1-A", listados1[0].Nome)
	assert.Equal(t, "Participante P1-B", listados1[1].Nome)
	assert.Equal(t, "Participante P2-A", listados2[0].Nome)
}
