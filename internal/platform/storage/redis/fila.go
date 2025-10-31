// Pacote redis implementa fila e contadores sobre Redis para acelerar leitura/escrita.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// Fila usa listas Redis para publicar e consumir votos de forma simples.
type Fila struct {
	client *redis.Client
	key    string
}

func NewFila(client *redis.Client, key string) *Fila {
	return &Fila{
		client: client,
		key:    key,
	}
}

func (f *Fila) PublicarVoto(ctx context.Context, voto domain.Voto) error {
	payload, err := json.Marshal(voto)
	if err != nil {
		return fmt.Errorf("redis fila: falha serializando voto: %w", err)
	}
	if err := f.client.LPush(ctx, f.key, payload).Err(); err != nil {
		return fmt.Errorf("redis fila: falha ao enfileirar voto: %w", err)
	}
	return nil
}

func (f *Fila) ConsumirVotos(ctx context.Context, handler func(context.Context, domain.Voto) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// BRPOP mantém o processamento bloqueado mas com timeout curto para respeitar o contexto.
		res, err := f.client.BRPop(ctx, 5*time.Second, f.key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("redis fila: falha ao consumir voto: %w", err)
		}

		if len(res) != 2 {
			continue
		}

		var voto domain.Voto
		if err := json.Unmarshal([]byte(res[1]), &voto); err != nil {
			return fmt.Errorf("redis fila: payload invalido: %w", err)
		}

		// Handler decide se o voto foi aceito; propagamos erro para interromper a rotina.
		if err := handler(ctx, voto); err != nil {
			return err
		}
	}
}

var _ domain.Fila = (*Fila)(nil)
