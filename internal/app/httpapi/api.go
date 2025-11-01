// Pacote httpapi expõe os handlers REST e traduz requisições HTTP para o serviço de votação.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/antifraude"
	"github.com/marcelojr/desafio-globo/internal/platform/metrics"
)

// API empacota handlers HTTP ligados ao serviço de votação e ao logger.
type API struct {
	service *voting.Service
	logger  *slog.Logger
}

func New(service *voting.Service, logger *slog.Logger) *API {
	return &API{service: service, logger: logger}
}

func (a *API) Register(mux *http.ServeMux) {
	// Mantemos as rotas centralizadas para facilitar testes e reuso em servidores diferentes.
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/paredoes", a.listarParedoes)
	mux.HandleFunc("/votos", a.handleVotos)
	mux.HandleFunc("/paredoes/", a.handleParedaoDetalhes)
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleParedaoDetalhes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/paredoes/")
	partes := strings.Split(path, "/")
	if len(partes) == 0 || partes[0] == "" {
		http.NotFound(w, r)
		return
	}

	id := domain.ParedaoID(partes[0])

	switch {
	case len(partes) == 1 && r.Method == http.MethodGet:
		a.obterParciais(w, r, id)
	case len(partes) == 2 && partes[1] == "hora" && r.Method == http.MethodGet:
		a.obterTotaisHora(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) handleVotos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "metodo nao suportado", http.StatusMethodNotAllowed)
		return
	}
	a.registrarVoto(w, r)
}

func (a *API) listarParedoes(w http.ResponseWriter, r *http.Request) {
	resultado, err := a.service.ListarAtivos(r.Context())
	if err != nil {
		a.logger.Error("erro ao listar paredoes", "err", err)
		responderErro(w, err)
		return
	}

	responderJSON(w, http.StatusOK, resultado)
}

type votoRequest struct {
	ParedaoID      string `json:"paredao_id"`
	ParticipanteID string `json:"participante_id"`
}

func (a *API) registrarVoto(w http.ResponseWriter, r *http.Request) {
	var req votoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.ObserveVoteRequest("invalid_payload")
		a.logger.Warn("payload invalido ao registrar voto", "err", err)
		http.Error(w, "payload invalido", http.StatusBadRequest)
		return
	}

	voto := domain.Voto{
		ParedaoID:      domain.ParedaoID(req.ParedaoID),
		ParticipanteID: domain.ParticipanteID(req.ParticipanteID),
		OrigemIP:       r.Header.Get("X-Forwarded-For"),
		UserAgent:      r.UserAgent(),
	}

	if voto.OrigemIP == "" {
		voto.OrigemIP = strings.Split(r.RemoteAddr, ":")[0]
	}

	if err := a.service.RegistrarVoto(r.Context(), voto); err != nil {
		status := statusFromError(err)
		metrics.ObserveVoteRequest(status)
		a.logger.Warn("falha ao registrar voto", "err", err, "paredao", req.ParedaoID, "participante", req.ParticipanteID, "status", status)
		responderErro(w, err)
		return
	}

	metrics.ObserveVoteRequest("accepted")
	responderJSON(w, http.StatusAccepted, map[string]string{"status": "recebido"})
	a.logger.Info("voto recebido", "paredao", req.ParedaoID, "participante", req.ParticipanteID)
}

func (a *API) obterParciais(w http.ResponseWriter, r *http.Request, id domain.ParedaoID) {
	parciais, err := a.service.Parciais(r.Context(), id)
	if err != nil {
		a.logger.Error("erro ao obter parciais", "err", err, "paredao", id)
		responderErro(w, err)
		return
	}

	responderJSON(w, http.StatusOK, parciais)
}

func (a *API) obterTotaisHora(w http.ResponseWriter, r *http.Request, id domain.ParedaoID) {
	totais, err := a.service.TotaisPorHora(r.Context(), id)
	if err != nil {
		a.logger.Error("erro ao obter totais por hora", "err", err, "paredao", id)
		responderErro(w, err)
		return
	}

	responderJSON(w, http.StatusOK, totais)
}

func responderJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func responderErro(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, voting.ErrParedaoInvalido):
		status = http.StatusBadRequest
	case errors.Is(err, voting.ErrParticipanteDesconhecido):
		status = http.StatusBadRequest
	case errors.Is(err, voting.ErrPeriodoEncerrado):
		status = http.StatusConflict
	case errors.Is(err, voting.ErrParedaoNaoEncontrado):
		status = http.StatusNotFound
	case errors.Is(err, antifraude.ErrRateLimitExceeded):
		status = http.StatusTooManyRequests
	}

	responderJSON(w, status, map[string]string{"erro": err.Error()})
}

func statusFromError(err error) string {
	switch {
	case errors.Is(err, antifraude.ErrRateLimitExceeded):
		return "rate_limited"
	case errors.Is(err, voting.ErrPeriodoEncerrado):
		return "closed"
	case errors.Is(err, voting.ErrParticipanteDesconhecido):
		return "invalid"
	case errors.Is(err, voting.ErrParedaoNaoEncontrado):
		return "not_found"
	default:
		return "error"
	}
}
