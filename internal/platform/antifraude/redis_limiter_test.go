package antifraude

import (
	"context"
	"errors"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/marcelojr/desafio-globo/internal/domain"
)

func TestRedisRateLimiterRespectsLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	limiter := NewRedisRateLimiter(client, 2, time.Minute, "rl")

	voto := domain.Voto{
		ParedaoID:      "paredao-1",
		ParticipanteID: "participante-1",
		OrigemIP:       "200.1.1.1",
		UserAgent:      "test-agent",
	}

	ctx := context.Background()
	if err := limiter.Validar(ctx, voto); err != nil {
		t.Fatalf("primeiro voto deveria ser aceito, erro: %v", err)
	}
	if err := limiter.Validar(ctx, voto); err != nil {
		t.Fatalf("segundo voto deveria ser aceito, erro: %v", err)
	}

	if err := limiter.Validar(ctx, voto); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("terceiro voto deveria ser bloqueado, recebeu: %v", err)
	}

	key := limiter.buildKey(voto)
	if ttl := mr.TTL(key); ttl <= 0 {
		t.Fatalf("esperava TTL positivo para %s, veio %v", key, ttl)
	}
}

func TestRedisRateLimiterResetsAfterWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	window := 30 * time.Second
	limiter := NewRedisRateLimiter(client, 1, window, "rl")

	voto := domain.Voto{
		ParedaoID:      "paredao-2",
		ParticipanteID: "participante-1",
		OrigemIP:       "200.2.2.2",
		UserAgent:      "ua",
	}

	ctx := context.Background()
	if err := limiter.Validar(ctx, voto); err != nil {
		t.Fatalf("voto inicial deveria ser aceito: %v", err)
	}
	if err := limiter.Validar(ctx, voto); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("segundo voto antes da janela deveria falhar: %v", err)
	}

	mr.FastForward(window + time.Second)

	if err := limiter.Validar(ctx, voto); err != nil {
		t.Fatalf("apos expirar janela, voto deveria ser aceito: %v", err)
	}
}
