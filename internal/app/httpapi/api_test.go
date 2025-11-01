package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/antifraude"
)

// MockVotingService implementa a interface do serviço de votação para testes
type MockVotingService struct {
	mock.Mock
}

func (m *MockVotingService) RegistrarVoto(ctx context.Context, voto domain.Voto) error {
	args := m.Called(ctx, voto)
	return args.Error(0)
}

func (m *MockVotingService) ListarAtivos(ctx context.Context) ([]domain.Paredao, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Paredao), args.Error(1)
}

func (m *MockVotingService) Parciais(ctx context.Context, id domain.ParedaoID) ([]domain.Parcial, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]domain.Parcial), args.Error(1)
}

func (m *MockVotingService) TotaisPorHora(ctx context.Context, id domain.ParedaoID) ([]domain.ParcialHora, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]domain.ParcialHora), args.Error(1)
}

func (m *MockVotingService) CriarParedao(ctx context.Context, paredao domain.Paredao, participantes []domain.Participante) (domain.Paredao, error) {
	args := m.Called(ctx, paredao, participantes)
	return args.Get(0).(domain.Paredao), args.Error(1)
}

// setupAPI cria uma instância da API com serviço mockado para testes
func setupAPI(t *testing.T) (*API, *MockVotingService) {
	mockService := new(MockVotingService)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{}))
	api := New(mockService, logger)

	t.Cleanup(func() {
		mockService.AssertExpectations(t)
	})

	return api, mockService
}

// === TESTES GET /healthz ===

func TestHandleHealthz_QuandoSolicitado_DeveRetornar200OK(t *testing.T) {
	api, _ := setupAPI(t)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	api.handleHealthz(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

// === TESTES GET /paredoes ===

func TestListarParedoes_QuandoExistemParedoesAtivos_DeveRetornarListaComSucesso(t *testing.T) {
	api, mockService := setupAPI(t)

	paredoes := []domain.Paredao{
		{ID: "01HXXXXXXXXXXXXXXXXXXXXX", Nome: "Paredão 1"},
		{ID: "01HXXXXXXXXXXXXXXXXXXXXY", Nome: "Paredão 2"},
	}

	mockService.On("ListarAtivos", mock.Anything).Return(paredoes, nil)

	req := httptest.NewRequest("GET", "/paredoes", nil)
	w := httptest.NewRecorder()

	api.listarParedoes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []domain.Paredao
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "Paredão 1", response[0].Nome)
	assert.Equal(t, "Paredão 2", response[1].Nome)
}

func TestListarParedoes_QuandoNaoExistemParedoes_DeveRetornarListaVazia(t *testing.T) {
	api, mockService := setupAPI(t)

	mockService.On("ListarAtivos", mock.Anything).Return([]domain.Paredao{}, nil)

	req := httptest.NewRequest("GET", "/paredoes", nil)
	w := httptest.NewRecorder()

	api.listarParedoes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []domain.Paredao
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 0)
}

func TestListarParedoes_QuandoServicoFalha_DeveRetornar500(t *testing.T) {
	api, mockService := setupAPI(t)

	mockService.On("ListarAtivos", mock.Anything).Return([]domain.Paredao(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/paredoes", nil)
	w := httptest.NewRecorder()

	api.listarParedoes(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

// === TESTES POST /votos ===

func TestRegistrarVoto_QuandoVotoValido_DeveRetornar202Accepted(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.MatchedBy(func(voto domain.Voto) bool {
		return string(voto.ParedaoID) == "01HXXXXXXXXXXXXXXXXXXXXX" &&
			string(voto.ParticipanteID) == "01HXXXXXXXXXXXXXXXXXXXXY"
	})).Return(nil)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "recebido", response["status"])
}

func TestRegistrarVoto_QuandoPayloadInvalido_DeveRetornar400BadRequest(t *testing.T) {
	api, _ := setupAPI(t)

	payload := `{"paredao_id":invalid}`

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "payload invalido\n", w.Body.String())
}

func TestRegistrarVoto_QuandoParedaoInvalido_DeveRetornar400BadRequest(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"invalid","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.Anything).Return(voting.ErrParedaoInvalido)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

func TestRegistrarVoto_QuandoParticipanteDesconhecido_DeveRetornar400BadRequest(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"unknown"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.Anything).Return(voting.ErrParticipanteDesconhecido)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

func TestRegistrarVoto_QuandoParedaoEncerrado_DeveRetornar409Conflict(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.Anything).Return(voting.ErrPeriodoEncerrado)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

func TestRegistrarVoto_QuandoRateLimitExcedido_DeveRetornar429TooManyRequests(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.Anything).Return(antifraude.ErrRateLimitExceeded)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

func TestRegistrarVoto_QuandoMetodoNaoSuportado_DeveRetornar405(t *testing.T) {
	api, _ := setupAPI(t)

	req := httptest.NewRequest("GET", "/votos", nil)
	w := httptest.NewRecorder()

	api.handleVotos(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Equal(t, "metodo nao suportado\n", w.Body.String())
}

func TestRegistrarVoto_QuandoXForwardedForPresente_DeveUsarComoOrigemIP(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.MatchedBy(func(voto domain.Voto) bool {
		return voto.OrigemIP == "192.168.1.100"
	})).Return(nil)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestRegistrarVoto_QuandoXForwardedForAusente_DeveUsarRemoteAddr(t *testing.T) {
	api, mockService := setupAPI(t)

	payload := `{"paredao_id":"01HXXXXXXXXXXXXXXXXXXXXX","participante_id":"01HXXXXXXXXXXXXXXXXXXXXY"}`
	mockService.On("RegistrarVoto", mock.Anything, mock.MatchedBy(func(voto domain.Voto) bool {
		return voto.OrigemIP == "127.0.0.1"
	})).Return(nil)

	req := httptest.NewRequest("POST", "/votos", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	api.registrarVoto(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// === TESTES GET /paredoes/{id} (Parciais) ===

func TestObterParciais_QuandoParedaoExiste_DeveRetornarParciais(t *testing.T) {
	api, mockService := setupAPI(t)

	paredaoID := domain.ParedaoID("01HXXXXXXXXXXXXXXXXXXXXX")
	parciais := []domain.Parcial{
		{ParedaoID: paredaoID, ParticipanteID: "01HXXXXXXXXXXXXXXXXXXXXY", Total: 100, Percentual: 50.0},
		{ParedaoID: paredaoID, ParticipanteID: "01HXXXXXXXXXXXXXXXXXXXXZ", Total: 100, Percentual: 50.0},
	}

	mockService.On("Parciais", mock.Anything, paredaoID).Return(parciais, nil)

	req := httptest.NewRequest("GET", "/paredoes/01HXXXXXXXXXXXXXXXXXXXXX", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []domain.Parcial
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, int64(100), response[0].Total)
	assert.Equal(t, 50.0, response[0].Percentual)
}

func TestObterParciais_QuandoParedaoNaoEncontrado_DeveRetornar404(t *testing.T) {
	api, mockService := setupAPI(t)

	paredaoID := domain.ParedaoID("01HXXXXXXXXXXXXXXXXXXXXX")
	mockService.On("Parciais", mock.Anything, paredaoID).Return([]domain.Parcial(nil), voting.ErrParedaoNaoEncontrado)

	req := httptest.NewRequest("GET", "/paredoes/01HXXXXXXXXXXXXXXXXXXXXX", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

func TestObterParciais_QuandoIDVazio_DeveRetornar404(t *testing.T) {
	api, _ := setupAPI(t)

	req := httptest.NewRequest("GET", "/paredoes/", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// === TESTES GET /paredoes/{id}/hora (Totais por Hora) ===

func TestObterTotaisHora_QuandoParedaoExiste_DeveRetornarTotaisHora(t *testing.T) {
	api, mockService := setupAPI(t)

	paredaoID := domain.ParedaoID("01HXXXXXXXXXXXXXXXXXXXXX")
	totais := []domain.ParcialHora{
		{ParedaoID: paredaoID, Hora: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), Total: 50},
		{ParedaoID: paredaoID, Hora: time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC), Total: 75},
	}

	mockService.On("TotaisPorHora", mock.Anything, paredaoID).Return(totais, nil)

	req := httptest.NewRequest("GET", "/paredoes/01HXXXXXXXXXXXXXXXXXXXXX/hora", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []domain.ParcialHora
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, int64(50), response[0].Total)
	assert.Equal(t, int64(75), response[1].Total)
}

func TestObterTotaisHora_QuandoParedaoNaoEncontrado_DeveRetornar404(t *testing.T) {
	api, mockService := setupAPI(t)

	paredaoID := domain.ParedaoID("01HXXXXXXXXXXXXXXXXXXXXX")
	mockService.On("TotaisPorHora", mock.Anything, paredaoID).Return([]domain.ParcialHora(nil), voting.ErrParedaoNaoEncontrado)

	req := httptest.NewRequest("GET", "/paredoes/01HXXXXXXXXXXXXXXXXXXXXX/hora", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response, "erro")
}

// === TESTES ROTAS NÃO ENCONTRADAS ===

func TestHandleParedaoDetalhes_QuandoRotaInvalida_DeveRetornar404(t *testing.T) {
	api, _ := setupAPI(t)

	req := httptest.NewRequest("GET", "/paredoes/01HXXXXXXXXXXXXXXXXXXXXX/invalid", nil)
	w := httptest.NewRecorder()

	api.handleParedaoDetalhes(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
