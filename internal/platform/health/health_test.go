package health

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupValidDB cria uma conexão SQLite válida para testes
func setupValidDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Criar uma tabela simples para validar conexão
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// setupInvalidDB cria uma conexão que será fechada para simular falha
func setupInvalidDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func setupMockRedis(t *testing.T) *redis.Client {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	return client
}

func TestReadyHandler_QuandoTodosServicosDisponiveis_DeveRetornar200OK(t *testing.T) {
	db := setupValidDB(t)
	redisClient := setupMockRedis(t)

	checker := NewChecker(db, redisClient)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestReadyHandler_QuandoDBENil_DevePularChecagem(t *testing.T) {
	redisClient := setupMockRedis(t)

	checker := NewChecker(nil, redisClient)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestReadyHandler_QuandoRedisENil_DevePularChecagem(t *testing.T) {
	db := setupValidDB(t)

	checker := NewChecker(db, nil)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestReadyHandler_QuandoAmbosNulos_DeveRetornar200(t *testing.T) {
	checker := NewChecker(nil, nil)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestReadyHandler_QuandoDBIndisponivel_DeveRetornar503(t *testing.T) {
	db := setupInvalidDB(t)
	redisClient := setupMockRedis(t)

	// Fechar DB para simular indisponibilidade
	db.Close()

	checker := NewChecker(db, redisClient)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "database unavailable\n", w.Body.String())
}

func TestReadyHandler_QuandoRedisIndisponivel_DeveRetornar503(t *testing.T) {
	db := setupValidDB(t)
	redisClient := setupMockRedis(t)

	// Fechar Redis para simular indisponibilidade
	redisClient.Close()

	checker := NewChecker(db, redisClient)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "redis unavailable\n", w.Body.String())
}

func TestReadyHandler_QuandoAmbosIndisponiveis_DeveRetornarPrimeiroErro(t *testing.T) {
	db := setupInvalidDB(t)
	redisClient := setupMockRedis(t)

	// Fechar ambos para simular indisponibilidade
	db.Close()
	redisClient.Close()

	checker := NewChecker(db, redisClient)
	handler := checker.ReadyHandler()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Deve retornar erro de DB primeiro (checado antes do Redis)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "database unavailable\n", w.Body.String())
}

func TestReadyHandler_QuandoContextoCancelado_DeveInterromper(t *testing.T) {
	db := setupValidDB(t)
	redisClient := setupMockRedis(t)

	checker := NewChecker(db, redisClient)
	handler := checker.ReadyHandler()

	// Criar request com contexto já cancelado
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar imediatamente

	req := httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "database unavailable\n", w.Body.String())
}
