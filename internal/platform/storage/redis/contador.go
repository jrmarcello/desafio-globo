package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

// Contador provê operações de incremento e leitura agregada usando chaves com prefixo.
type Contador struct {
	client *redis.Client
	prefix string
}

func NewContador(client *redis.Client, prefix string) *Contador {
	return &Contador{
		client: client,
		prefix: prefix,
	}
}

func (c *Contador) Incrementar(ctx context.Context, chave string, delta int64) (int64, error) {
	// Incremento simples no Redis mantém leitura barata para as parciais.
	return c.client.IncrBy(ctx, c.key(chave), delta).Result()
}

func (c *Contador) Obter(ctx context.Context, chave string) (int64, error) {
	val, err := c.client.Get(ctx, c.key(chave)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (c *Contador) ObterTodos(ctx context.Context, chaves []string) (map[string]int64, error) {
	if len(chaves) == 0 {
		return map[string]int64{}, nil
	}

	keys := make([]string, len(chaves))
	for i, ch := range chaves {
		keys[i] = c.key(ch)
	}

	// MGET reduz round-trips quando precisamos das parciais completas.
	valores, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	resultado := make(map[string]int64, len(chaves))
	for i, raw := range valores {
		if raw == nil {
			resultado[chaves[i]] = 0
			continue
		}

		switch v := raw.(type) {
		case string:
			num, convErr := strconv.ParseInt(v, 10, 64)
			if convErr != nil {
				return nil, fmt.Errorf("redis contador: valor invalido para %s: %w", chaves[i], convErr)
			}
			resultado[chaves[i]] = num
		case int64:
			resultado[chaves[i]] = v
		default:
			return nil, fmt.Errorf("redis contador: tipo inesperado %T", raw)
		}
	}

	return resultado, nil
}

func (c *Contador) key(chave string) string {
	if c.prefix == "" {
		return chave
	}
	return fmt.Sprintf("%s:%s", c.prefix, chave)
}

var _ domain.Contador = (*Contador)(nil)
