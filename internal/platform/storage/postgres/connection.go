// Pacote postgres implementa a camada de persistência no Postgres via GORM.
package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	// Configuração mínima: nomes padrão e logs somente em WARN para evitar ruído.
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false,
		},
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres gorm: abrir conexao: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres gorm: obter sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(60 * time.Minute)

	// Ping inicial garante que a instância está acessível antes de devolver a conexão.
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctxPing); err != nil {
		return nil, fmt.Errorf("postgres gorm: ping falhou: %w", err)
	}

	return gormDB, nil
}
