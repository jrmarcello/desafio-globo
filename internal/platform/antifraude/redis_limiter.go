// Pacote antifraude oferece implementações para controle de votos suspeitos (rate limit Redis e modo noop).
package antifraude

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

var ErrRateLimitExceeded = fmt.Errorf("limite de votos atingido")

// RedisRateLimiter limita votos por IP/UA em janelas fixas usando Redis.
type RedisRateLimiter struct {
	client    *redis.Client
	limit     int
	window    time.Duration
	keyPrefix string
}

func NewRedisRateLimiter(client *redis.Client, limit int, window time.Duration, prefix string) *RedisRateLimiter {
	if prefix == "" {
		prefix = "ratelimit"
	}
	return &RedisRateLimiter{
		client:    client,
		limit:     limit,
		window:    window,
		keyPrefix: prefix,
	}
}

func (r *RedisRateLimiter) Validar(ctx context.Context, voto domain.Voto) error {
	if r.client == nil || r.limit <= 0 || r.window <= 0 {
		// Configurações inválidas caem automaticamente no modo permissivo.
		return nil
	}

	key := r.buildKey(voto)
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("antifraude: falha ao incrementar chave: %w", err)
	}

	if count == 1 {
		if err := r.client.Expire(ctx, key, r.window).Err(); err != nil {
			return fmt.Errorf("antifraude: falha ao definir expiracao: %w", err)
		}
	}

	if int(count) > r.limit {
		return ErrRateLimitExceeded
	}

	return nil
}

func (r *RedisRateLimiter) buildKey(voto domain.Voto) string {
	// Hash SHA-1 evita expor IP/UA diretamente no Redis e mantém o prefixo limpo.
	base := fmt.Sprintf("%s|%s|%s", voto.ParedaoID, voto.OrigemIP, voto.UserAgent)
	hash := sha1.Sum([]byte(base))
	return fmt.Sprintf("%s:%s", r.keyPrefix, hex.EncodeToString(hash[:]))
}

var _ domain.Antifraude = (*RedisRateLimiter)(nil)
