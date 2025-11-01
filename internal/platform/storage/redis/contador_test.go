package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	return client, mr
}

func TestContador_IncrementarEObter_QuandoChaveNova_DeveRetornarValorIncrementado(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "contador")

	ctx := context.Background()
	chave := "paredao:01HXXXXXXXXXXXXXXXXXXXXX"

	// Act
	resultado, err := repo.Incrementar(ctx, chave, 1)
	require.NoError(t, err)

	valor, err := repo.Obter(ctx, chave)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resultado)
	assert.Equal(t, int64(1), valor)
}

func TestContador_Incrementar_QuandoMultiplasChamadas_DeveAcumular(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "contador")

	ctx := context.Background()
	chave := "participante:01HXXXXXXXXXXXXXXXXXXXXX"

	// Act: Incrementar 3 vezes
	resultado1, err := repo.Incrementar(ctx, chave, 1)
	require.NoError(t, err)

	resultado2, err := repo.Incrementar(ctx, chave, 2)
	require.NoError(t, err)

	resultado3, err := repo.Incrementar(ctx, chave, 1)
	require.NoError(t, err)

	valorFinal, err := repo.Obter(ctx, chave)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resultado1)
	assert.Equal(t, int64(3), resultado2)
	assert.Equal(t, int64(4), resultado3)
	assert.Equal(t, int64(4), valorFinal)
}

func TestContador_Obter_QuandoChaveNaoExiste_DeveRetornarZero(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "contador")

	ctx := context.Background()
	chave := "inexistente"

	// Act
	valor, err := repo.Obter(ctx, chave)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(0), valor)
}

func TestContador_ObterTodos_QuandoChavesExistem_DeveRetornarMapaCompleto(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "contador")

	ctx := context.Background()
	chaves := []string{"chave1", "chave2", "chave3"}

	// Arrange: Set some values
	_, err := repo.Incrementar(ctx, chaves[0], 5)
	require.NoError(t, err)

	_, err = repo.Incrementar(ctx, chaves[1], 10)
	require.NoError(t, err)

	// chave2 não existe, deve retornar 0

	// Act
	resultado, err := repo.ObterTodos(ctx, chaves)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(5), resultado[chaves[0]])
	assert.Equal(t, int64(10), resultado[chaves[1]])
	assert.Equal(t, int64(0), resultado[chaves[2]])
}

func TestContador_ObterTodos_QuandoListaVazia_DeveRetornarMapaVazio(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "contador")

	ctx := context.Background()
	var chaves []string

	// Act
	resultado, err := repo.ObterTodos(ctx, chaves)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, resultado)
}

func TestContador_key_QuandoPrefixVazio_DeveRetornarChaveSemPrefixo(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "")

	chave := "minha-chave"
	resultado := repo.key(chave)

	assert.Equal(t, "minha-chave", resultado)
}

func TestContador_key_QuandoPrefixExiste_DeveRetornarChaveComPrefixo(t *testing.T) {
	client, _ := setupRedis(t)
	repo := NewContador(client, "prefixo")

	chave := "minha-chave"
	resultado := repo.key(chave)

	assert.Equal(t, "prefixo:minha-chave", resultado)
}
