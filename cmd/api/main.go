// Executável principal da API: carrega a configuração, inicializa dependências e sobe o servidor HTTP.
package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/marcelojr/desafio-globo/internal/app/httpapi"
	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/app/web"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/antifraude"
	"github.com/marcelojr/desafio-globo/internal/platform/clock"
	"github.com/marcelojr/desafio-globo/internal/platform/config"
	"github.com/marcelojr/desafio-globo/internal/platform/health"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
	"github.com/marcelojr/desafio-globo/internal/platform/logger"
	"github.com/marcelojr/desafio-globo/internal/platform/migrations"
	postgresstorage "github.com/marcelojr/desafio-globo/internal/platform/storage/postgres"
	redisstorage "github.com/marcelojr/desafio-globo/internal/platform/storage/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("configuracao invalida", "err", err)
	}

	// Mantemos a conexão compartilhada em todo o ciclo para reaproveitar pool e checar readiness.
	db, err := postgresstorage.Open(ctx, cfg.PostgresDSN())
	if err != nil {
		logger.Fatal("falha ao conectar no postgres", "err", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("falha ao resgatar sql.DB", "err", err)
	}
	defer sqlDB.Close()

	if cfg.AutoMigrate {
		// Rodamos migrations automáticas apenas se habilitado para evitar surpresas em produção.
		if err := migrations.Run(db); err != nil {
			logger.Fatal("falha na migracao automatica", "err", err)
		}
	}

	// Redis centraliza fila, contadores e antifraude; sem ele o sistema não processa votos.
	redisClient, err := redisstorage.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Fatal("falha ao conectar no redis", "err", err)
	}
	defer redisClient.Close()

	dbParedao := postgresstorage.NewParedaoRepository(db)
	dbParticipante := postgresstorage.NewParticipanteRepository(db)
	dbVoto := postgresstorage.NewVotoRepository(db)
	contador := redisstorage.NewContador(redisClient, cfg.ContadorKeyPrefix)
	fila := redisstorage.NewFila(redisClient, cfg.FilaKeyPrefix)
	clockSystem := clock.NewSystemClock()
	idGen := ids.NewGenerator()

	var antifraudeSvc domain.Antifraude = antifraude.NewNoop()
	if cfg.RateLimitEnabled {
		window := time.Duration(cfg.RateLimitWindowSeconds) * time.Second
		antifraudeSvc = antifraude.NewRedisRateLimiter(redisClient, cfg.RateLimitMaxActions, window, cfg.RateLimitKeyPrefix)
	}

	// Serviço agrega repositórios, fila e antifraude para guardar a lógica de negócio.
	servico := voting.NewService(
		dbParedao,
		dbParticipante,
		dbVoto,
		contador,
		fila,
		antifraudeSvc,
		clockSystem,
		idGen,
	)

	mux := http.NewServeMux()
	checker := health.NewChecker(sqlDB, redisClient)

	// HTTP expõe API, health check e métricas que o Prometheus coleta.
	api := httpapi.New(servico, logger.L())
	api.Register(mux)
	frontend, err := web.New(servico, cfg.ConsultaToken)
	if err != nil {
		logger.Fatal("erro ao carregar templates", "err", err)
	}
	frontend.Register(mux)
	mux.HandleFunc("/readyz", checker.ReadyHandler())
	mux.Handle("/metrics", promhttp.Handler())

	logger.Info("api ouvindo", "addr", cfg.HTTPAddress)
	if err := http.ListenAndServe(cfg.HTTPAddress, mux); err != nil {
		logger.Fatal("erro no servidor", "err", err)
	}
}
