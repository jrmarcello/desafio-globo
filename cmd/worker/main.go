// Worker assíncrono que consome votos da fila, persiste no Postgres e mantém métricas expostas.
package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/marcelojr/desafio-globo/internal/app/worker"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/clock"
	"github.com/marcelojr/desafio-globo/internal/platform/config"
	"github.com/marcelojr/desafio-globo/internal/platform/health"
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

	// Worker usa a mesma conexão GORM da API para compartilhar migrations e modelos.
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
		// Evitamos divergência de schema rodando a mesma migração condicional da API.
		if err := migrations.Run(db); err != nil {
			logger.Fatal("falha na migracao automatica", "err", err)
		}
	}

	// Redis é obrigatório aqui porque fila e contador vivem sobre a mesma instância.
	redisClient, err := redisstorage.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Fatal("falha ao conectar no redis", "err", err)
	}
	defer redisClient.Close()

	contador := redisstorage.NewContador(redisClient, cfg.ContadorKeyPrefix)
	fila := redisstorage.NewFila(redisClient, cfg.FilaKeyPrefix)
	clockSystem := clock.NewSystemClock()
	checker := health.NewChecker(sqlDB, redisClient)

	if cfg.WorkerMetricsAddress != "" {
		go func() {
			// Metrics expõe observabilidade enquanto a goroutine principal consome a fila.
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			mux.HandleFunc("/readyz", checker.ReadyHandler())
			logger.Info("worker metrics ouvindo", "addr", cfg.WorkerMetricsAddress)
			if err := http.ListenAndServe(cfg.WorkerMetricsAddress, mux); err != nil {
				logger.Error("erro no servidor de metrics do worker", "err", err)
			}
		}()
	}

	votoRepo := postgresstorage.NewVotoRepository(db)
	processor := worker.NewVoteProcessor(votoRepo, contador, clockSystem)

	logger.Info("worker iniciado, aguardando votos")
	err = fila.ConsumirVotos(ctx, func(ctx context.Context, voto domain.Voto) error {
		// Processamos voto a voto para manter a semântica de uma fila simples.
		if err := processor.Process(ctx, voto); err != nil {
			logger.Error("erro ao processar voto", "voto", voto.ID, "err", err)
		}
		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.Fatal("worker finalizado com erro", "err", err)
	}

	logger.Info("worker finalizado")
}
