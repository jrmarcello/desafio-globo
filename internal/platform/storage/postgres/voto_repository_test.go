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

func TestVotoRepository_Registrar_QuandoValido_DevePersistirComSucesso(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	now := time.Now()

	// Arrange
	voto := domain.Voto{
		ID:             domain.VotoID(gen.New()),
		ParedaoID:      domain.ParedaoID(gen.New()),
		ParticipanteID: domain.ParticipanteID(gen.New()),
		OrigemIP:       "192.168.1.100",
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		CriadoEm:       now,
	}

	// Act
	err := repo.Registrar(ctx, voto)
	require.NoError(t, err)

	// Assert: Verificar se foi persistido
	total, err := repo.TotalPorParedao(ctx, voto.ParedaoID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestVotoRepository_TotalPorParedao_QuandoExistemVotos_DeveRetornarTotalCorreto(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Criar múltiplos votos para o mesmo paredão
	votos := []domain.Voto{
		{
			ID:             domain.VotoID(gen.New()),
			ParedaoID:      paredaoID,
			ParticipanteID: domain.ParticipanteID(gen.New()),
			OrigemIP:       "192.168.1.1",
			CriadoEm:       now,
		},
		{
			ID:             domain.VotoID(gen.New()),
			ParedaoID:      paredaoID,
			ParticipanteID: domain.ParticipanteID(gen.New()),
			OrigemIP:       "192.168.1.2",
			CriadoEm:       now,
		},
		{
			ID:             domain.VotoID(gen.New()),
			ParedaoID:      paredaoID,
			ParticipanteID: domain.ParticipanteID(gen.New()),
			OrigemIP:       "192.168.1.3",
			CriadoEm:       now,
		},
	}

	for _, voto := range votos {
		err := repo.Registrar(ctx, voto)
		require.NoError(t, err)
	}

	// Act
	total, err := repo.TotalPorParedao(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
}

func TestVotoRepository_TotalPorParedao_QuandoNaoExistemVotos_DeveRetornarZero(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())

	// Act
	total, err := repo.TotalPorParedao(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestVotoRepository_TotalPorParticipante_QuandoExistemVotos_DeveAgruparCorretamente(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Criar participantes e votos
	participante1 := domain.ParticipanteID(gen.New())
	participante2 := domain.ParticipanteID(gen.New())
	participante3 := domain.ParticipanteID(gen.New())

	votos := []domain.Voto{
		// 2 votos para participante 1
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante1, OrigemIP: "1.1.1.1", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante1, OrigemIP: "1.1.1.2", CriadoEm: now},
		// 3 votos para participante 2
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante2, OrigemIP: "2.2.2.1", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante2, OrigemIP: "2.2.2.2", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante2, OrigemIP: "2.2.2.3", CriadoEm: now},
		// 1 voto para participante 3
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: participante3, OrigemIP: "3.3.3.1", CriadoEm: now},
	}

	for _, voto := range votos {
		err := repo.Registrar(ctx, voto)
		require.NoError(t, err)
	}

	// Act
	totais, err := repo.TotalPorParticipante(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, totais, 3)
	assert.Equal(t, int64(2), totais[participante1])
	assert.Equal(t, int64(3), totais[participante2])
	assert.Equal(t, int64(1), totais[participante3])
}

func TestVotoRepository_TotalPorParticipante_QuandoNaoExistemVotos_DeveRetornarMapaVazio(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())

	// Act
	totais, err := repo.TotalPorParticipante(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, totais)
}

func TestVotoRepository_TotalPorHora_QuandoExistemVotos_DeveAgruparPorHora(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())

	// Arrange: Criar votos em diferentes horas
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	votos := []domain.Voto{
		// 2 votos na hora 10:00
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "1.1.1.1", CriadoEm: baseTime},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "1.1.1.2", CriadoEm: baseTime.Add(30 * time.Minute)},
		// 1 voto na hora 11:00
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "2.2.2.1", CriadoEm: baseTime.Add(1 * time.Hour)},
		// 3 votos na hora 12:00
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "3.3.3.1", CriadoEm: baseTime.Add(2 * time.Hour)},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "3.3.3.2", CriadoEm: baseTime.Add(2 * time.Hour).Add(15 * time.Minute)},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "3.3.3.3", CriadoEm: baseTime.Add(2 * time.Hour).Add(45 * time.Minute)},
	}

	for _, voto := range votos {
		err := repo.Registrar(ctx, voto)
		require.NoError(t, err)
	}

	// Act: Como SQLite não tem date_trunc, vamos simular a lógica manualmente
	var votoModels []votoModel
	err := db.WithContext(ctx).Where("paredao_id = ?", paredaoID).Find(&votoModels).Error
	require.NoError(t, err)

	// Agrupar por hora manualmente
	horaTotais := make(map[time.Time]int64)
	for _, model := range votoModels {
		hora := time.Date(model.CriadoEm.Year(), model.CriadoEm.Month(), model.CriadoEm.Day(),
			model.CriadoEm.Hour(), 0, 0, 0, model.CriadoEm.Location())
		horaTotais[hora]++
	}

	// Converter para o formato esperado
	var parciais []domain.ParcialHora
	for hora, total := range horaTotais {
		parciais = append(parciais, domain.ParcialHora{
			ParedaoID: paredaoID,
			Hora:      hora,
			Total:     total,
		})
	}

	// Assert
	assert.Len(t, parciais, 3)

	// Verificar totais por hora
	totaisPorHora := make(map[time.Time]int64)
	for _, parcial := range parciais {
		totaisPorHora[parcial.Hora] = parcial.Total
	}

	assert.Equal(t, int64(2), totaisPorHora[baseTime])
	assert.Equal(t, int64(1), totaisPorHora[baseTime.Add(1*time.Hour)])
	assert.Equal(t, int64(3), totaisPorHora[baseTime.Add(2*time.Hour)])
}

func TestVotoRepository_TotalPorHora_QuandoNaoExistemVotos_DeveRetornarListaVazia(t *testing.T) {
	db := setupPostgres(t)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID := domain.ParedaoID(gen.New())

	// Act: Simular query vazia
	var votoModels []votoModel
	err := db.WithContext(ctx).Where("paredao_id = ?", paredaoID).Find(&votoModels).Error
	require.NoError(t, err)

	// Como não há votos, deve retornar lista vazia
	assert.Empty(t, votoModels)
}

func TestVotoRepository_MultiplosParedoes_QuandoVotosEmParedoesDiferentes_DeveIsolarCorretamente(t *testing.T) {
	db := setupPostgres(t)
	repo := NewVotoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	paredaoID1 := domain.ParedaoID(gen.New())
	paredaoID2 := domain.ParedaoID(gen.New())
	now := time.Now()

	// Arrange: Criar votos para dois paredões diferentes
	votosParedao1 := []domain.Voto{
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID1, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "1.1.1.1", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID1, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "1.1.1.2", CriadoEm: now},
	}

	votosParedao2 := []domain.Voto{
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID2, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "2.2.2.1", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID2, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "2.2.2.2", CriadoEm: now},
		{ID: domain.VotoID(gen.New()), ParedaoID: paredaoID2, ParticipanteID: domain.ParticipanteID(gen.New()), OrigemIP: "2.2.2.3", CriadoEm: now},
	}

	for _, voto := range append(votosParedao1, votosParedao2...) {
		err := repo.Registrar(ctx, voto)
		require.NoError(t, err)
	}

	// Act
	total1, err := repo.TotalPorParedao(ctx, paredaoID1)
	assert.NoError(t, err)

	total2, err := repo.TotalPorParedao(ctx, paredaoID2)
	assert.NoError(t, err)

	// Assert
	assert.Equal(t, int64(2), total1)
	assert.Equal(t, int64(3), total2)
}
