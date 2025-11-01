package redis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

func TestFila_PublicarVotoEConsumir_QuandoValido_DeveProcessarComSucesso(t *testing.T) {
	client, _ := setupRedis(t)
	fila := NewFila(client, "votos:queue")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Arrange: Criar voto de teste
	gen := ids.NewGenerator()
	voto := domain.Voto{
		ID:             domain.VotoID(gen.New()),
		ParedaoID:      domain.ParedaoID(gen.New()),
		ParticipanteID: domain.ParticipanteID(gen.New()),
		OrigemIP:       "192.168.1.1",
		UserAgent:      "Mozilla/5.0...",
	}

	var votoRecebido *domain.Voto
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		handler := func(ctx context.Context, v domain.Voto) error {
			mu.Lock()
			votoRecebido = &v
			mu.Unlock()
			return nil // Processado com sucesso
		}

		err := fila.ConsumirVotos(ctx, handler)
		if err != nil && err != context.DeadlineExceeded {
			t.Errorf("Erro inesperado no consumo: %v", err)
		}
	}()

	// Pequena pausa para garantir que o consumidor está esperando
	time.Sleep(100 * time.Millisecond)

	// Act: Publicar voto
	err := fila.PublicarVoto(ctx, voto)
	require.NoError(t, err)

	// Aguardar processamento
	wg.Wait()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, votoRecebido)
	assert.Equal(t, voto.ID, votoRecebido.ID)
	assert.Equal(t, voto.ParedaoID, votoRecebido.ParedaoID)
	assert.Equal(t, voto.ParticipanteID, votoRecebido.ParticipanteID)
	assert.Equal(t, voto.OrigemIP, votoRecebido.OrigemIP)
	assert.Equal(t, voto.UserAgent, votoRecebido.UserAgent)
}

func TestFila_PublicarVoto_QuandoMultiplosVotos_DeveProcessarTodos(t *testing.T) {
	client, _ := setupRedis(t)
	fila := NewFila(client, "votos:queue")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	gen := ids.NewGenerator()
	votos := []domain.Voto{
		{
			ID:             domain.VotoID(gen.New()),
			ParedaoID:      domain.ParedaoID(gen.New()),
			ParticipanteID: domain.ParticipanteID(gen.New()),
			OrigemIP:       "192.168.1.1",
			UserAgent:      "Mozilla/5.0...",
		},
		{
			ID:             domain.VotoID(gen.New()),
			ParedaoID:      domain.ParedaoID(gen.New()),
			ParticipanteID: domain.ParticipanteID(gen.New()),
			OrigemIP:       "192.168.1.2",
			UserAgent:      "Chrome/91.0...",
		},
	}

	var votosRecebidos []domain.Voto
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		handler := func(ctx context.Context, v domain.Voto) error {
			mu.Lock()
			votosRecebidos = append(votosRecebidos, v)
			mu.Unlock()

			// Parar após receber todos os votos esperados
			if len(votosRecebidos) >= len(votos) {
				return errors.New("processamento concluído")
			}
			return nil
		}

		err := fila.ConsumirVotos(ctx, handler)
		if err != nil && err.Error() != "processamento concluído" {
			t.Errorf("Erro inesperado no consumo: %v", err)
		}
	}()

	// Pequena pausa para garantir que o consumidor está esperando
	time.Sleep(100 * time.Millisecond)

	// Act: Publicar votos
	for _, voto := range votos {
		err := fila.PublicarVoto(ctx, voto)
		require.NoError(t, err)
	}

	// Aguardar processamento
	wg.Wait()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, votosRecebidos, len(votos))

	// Verificar que todos os votos foram recebidos (ordem pode variar)
	recebidosIDs := make(map[domain.VotoID]bool)
	for _, v := range votosRecebidos {
		recebidosIDs[v.ID] = true
	}

	for _, voto := range votos {
		assert.True(t, recebidosIDs[voto.ID], "Voto %s não foi recebido", voto.ID)
	}
}

func TestFila_ConsumirVotos_QuandoFilaVazia_DeveAguardar(t *testing.T) {
	client, _ := setupRedis(t)
	fila := NewFila(client, "votos:queue")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var votosRecebidos []domain.Voto
	handler := func(ctx context.Context, v domain.Voto) error {
		votosRecebidos = append(votosRecebidos, v)
		return nil
	}

	// Act
	err := fila.ConsumirVotos(ctx, handler)

	// Assert: Deve terminar por timeout, não por erro
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Empty(t, votosRecebidos)
}

func TestFila_ConsumirVotos_QuandoContextoCancelado_DeveParar(t *testing.T) {
	client, _ := setupRedis(t)
	fila := NewFila(client, "votos:queue")

	ctx, cancel := context.WithCancel(context.Background())

	var votosRecebidos []domain.Voto
	handler := func(ctx context.Context, v domain.Voto) error {
		votosRecebidos = append(votosRecebidos, v)
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := fila.ConsumirVotos(ctx, handler)
		assert.Equal(t, context.Canceled, err)
	}()

	// Cancelar contexto imediatamente
	cancel()

	// Aguardar
	wg.Wait()

	// Assert
	assert.Empty(t, votosRecebidos)
}
