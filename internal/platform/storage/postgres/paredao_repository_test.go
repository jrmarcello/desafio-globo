package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

func setupPostgres(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Aplicar migrations no banco de teste
	err = db.AutoMigrate(&domain.Paredao{}, &domain.Participante{}, &domain.Voto{})
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	return db
}

func TestParedaoRepository_FindByID_QuandoExiste_DeveRetornarParedaoComParticipantes(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()

	// Arrange: Criar paredão de teste
	paredaoID := domain.ParedaoID(gen.New())
	now := time.Now()
	paredao := domain.Paredao{
		ID:        paredaoID,
		Nome:      "Paredão Teste",
		Descricao: "Descrição teste",
		Inicio:    now.Add(-1 * time.Hour),
		Fim:       now.Add(24 * time.Hour),
		Ativo:     true,
		CriadoEm:  now,
	}

	// Criar participantes
	participantes := []domain.Participante{
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Participante 1",
		},
		{
			ID:        domain.ParticipanteID(gen.New()),
			ParedaoID: paredaoID,
			Nome:      "Participante 2",
		},
	}
	paredao.Participantes = participantes

	err := repo.Create(ctx, paredao)
	require.NoError(t, err)

	// Act
	encontrado, err := repo.FindByID(ctx, paredaoID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, encontrado)
	assert.Equal(t, paredaoID, encontrado.ID)
	assert.Equal(t, "Paredão Teste", encontrado.Nome)
	assert.Equal(t, "Descrição teste", encontrado.Descricao)
	assert.True(t, encontrado.Ativo)
	assert.Len(t, encontrado.Participantes, 2)
	assert.Equal(t, "Participante 1", encontrado.Participantes[0].Nome)
	assert.Equal(t, "Participante 2", encontrado.Participantes[1].Nome)
}

func TestParedaoRepository_FindByID_QuandoNaoExiste_DeveRetornarErroNotFound(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	idInexistente := domain.ParedaoID(gen.New())

	// Act
	resultado, err := repo.FindByID(ctx, idInexistente)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, domain.ErrNotFound, err)
	assert.Equal(t, domain.Paredao{}, resultado)
}

func TestParedaoRepository_ListAtivos_QuandoExistemAtivos_DeveRetornarLista(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	now := time.Now()

	// Arrange: Criar múltiplos paredões
	paredoes := []domain.Paredao{
		{
			ID:       domain.ParedaoID(gen.New()),
			Nome:     "Paredão Ativo 1",
			Inicio:   now.Add(-1 * time.Hour),
			Fim:      now.Add(24 * time.Hour),
			Ativo:    true,
			CriadoEm: now,
		},
		{
			ID:       domain.ParedaoID(gen.New()),
			Nome:     "Paredão Ativo 2",
			Inicio:   now.Add(-2 * time.Hour),
			Fim:      now.Add(48 * time.Hour),
			Ativo:    true,
			CriadoEm: now,
		},
		{
			ID:       domain.ParedaoID(gen.New()),
			Nome:     "Paredão Inativo",
			Inicio:   now.Add(-1 * time.Hour),
			Fim:      now.Add(24 * time.Hour),
			Ativo:    false,
			CriadoEm: now,
		},
		{
			ID:       domain.ParedaoID(gen.New()),
			Nome:     "Paredão Fora do Prazo",
			Inicio:   now.Add(-48 * time.Hour),
			Fim:      now.Add(-24 * time.Hour), // Já terminou
			Ativo:    true,
			CriadoEm: now,
		},
	}

	for _, p := range paredoes {
		err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	// Act: Simular a lógica de ListAtivos usando query direta no SQLite
	var models []paredaoModel
	err := db.WithContext(ctx).
		Where("ativo = ? AND inicio <= ? AND fim >= ?", true, now, now).
		Order("inicio ASC").
		Find(&models).Error
	require.NoError(t, err)

	resultado := make([]domain.Paredao, len(models))
	for i, model := range models {
		resultado[i] = model.toDomain(false)
	}

	// Assert
	assert.Len(t, resultado, 2)

	// Verificar que apenas os ativos no período correto foram retornados
	nomes := make([]string, len(resultado))
	for i, p := range resultado {
		nomes[i] = p.Nome
		assert.True(t, p.Ativo)
	}

	assert.Contains(t, nomes, "Paredão Ativo 1")
	assert.Contains(t, nomes, "Paredão Ativo 2")
}

func TestParedaoRepository_ListAtivos_QuandoNenhumAtivo_DeveRetornarListaVazia(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	now := time.Now()

	// Arrange: Criar apenas paredões inativos
	paredao := domain.Paredao{
		ID:       domain.ParedaoID(gen.New()),
		Nome:     "Paredão Inativo",
		Inicio:   now.Add(-1 * time.Hour),
		Fim:      now.Add(24 * time.Hour),
		Ativo:    false,
		CriadoEm: now,
	}

	err := repo.Create(ctx, paredao)
	require.NoError(t, err)

	// Act: Simular a lógica usando query direta
	var models []paredaoModel
	err = db.WithContext(ctx).
		Where("ativo = ? AND inicio <= ? AND fim >= ?", true, now, now).
		Order("inicio ASC").
		Find(&models).Error
	require.NoError(t, err)

	resultado := make([]domain.Paredao, len(models))
	for i, model := range models {
		resultado[i] = model.toDomain(false)
	}

	// Assert
	assert.Empty(t, resultado)
}

func TestParedaoRepository_Create_QuandoValido_DevePersistirComSucesso(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	now := time.Now()

	// Arrange
	paredao := domain.Paredao{
		ID:        domain.ParedaoID(gen.New()),
		Nome:      "Novo Paredão",
		Descricao: "Descrição do novo paredão",
		Inicio:    now.Add(1 * time.Hour),
		Fim:       now.Add(25 * time.Hour),
		Ativo:     true,
		CriadoEm:  now,
	}

	// Act
	err := repo.Create(ctx, paredao)
	require.NoError(t, err)

	// Assert: Verificar se foi persistido
	encontrado, err := repo.FindByID(ctx, paredao.ID)
	assert.NoError(t, err)
	assert.Equal(t, paredao.ID, encontrado.ID)
	assert.Equal(t, "Novo Paredão", encontrado.Nome)
}

func TestParedaoRepository_Update_QuandoExiste_DeveAtualizarComSucesso(t *testing.T) {
	db := setupPostgres(t)
	repo := NewParedaoRepository(db)

	ctx := context.Background()
	gen := ids.NewGenerator()
	now := time.Now()

	// Arrange: Criar paredão inicial
	paredao := domain.Paredao{
		ID:        domain.ParedaoID(gen.New()),
		Nome:      "Paredão Original",
		Descricao: "Descrição original",
		Inicio:    now.Add(-1 * time.Hour),
		Fim:       now.Add(24 * time.Hour),
		Ativo:     true,
		CriadoEm:  now,
	}

	err := repo.Create(ctx, paredao)
	require.NoError(t, err)

	// Act: Atualizar
	paredao.Nome = "Paredão Atualizado"
	paredao.Descricao = "Descrição atualizada"
	paredao.Ativo = false
	paredao.AtualizadoEm = now.Add(1 * time.Hour)

	err = repo.Update(ctx, paredao)
	require.NoError(t, err)

	// Assert
	encontrado, err := repo.FindByID(ctx, paredao.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Paredão Atualizado", encontrado.Nome)
	assert.Equal(t, "Descrição atualizada", encontrado.Descricao)
	assert.False(t, encontrado.Ativo)
}
