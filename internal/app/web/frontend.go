package web

// Pacote web centraliza a camada de apresentação HTML (SSR) usada pelo desafio.

import (
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marcelojr/desafio-globo/internal/app/voting"
	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/antifraude"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// Frontend renderiza os templates Go responsáveis pelas telas de voto, panorama e consulta.
type Frontend struct {
	templates     *template.Template
	service       *voting.Service
	consultaToken string
}

// New carrega os templates embutidos e registra as dependências necessárias.
func New(service *voting.Service, consultaToken string) (*Frontend, error) {
	if service == nil {
		return nil, fmt.Errorf("frontend: serviço de votação inexistente")
	}
	tmpl, err := template.ParseFS(templateFS,
		"templates/layout.gohtml",
		"templates/vote.gohtml",
		"templates/panorama.gohtml",
		"templates/consulta.gohtml",
	)
	if err != nil {
		return nil, err
	}

	for _, name := range []string{"vote_body", "panorama_body", "consulta_body", "layout"} {
		if tmpl.Lookup(name) == nil {
			return nil, fmt.Errorf("frontend: template %s não encontrado", name)
		}
	}

	return &Frontend{templates: tmpl, service: service, consultaToken: consultaToken}, nil
}

// Register expõe as rotas HTML na mesma mux da API.
func (f *Frontend) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", f.handleRoot)
	mux.HandleFunc("/vote", f.handleVote)
	mux.HandleFunc("/panorama", f.handlePanorama)
	mux.HandleFunc("/consulta", f.handleConsulta)
}

func (f *Frontend) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/vote", http.StatusFound)
}

func (f *Frontend) handleVote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := votePageData{}

	paredoes, err := f.service.ListarAtivos(ctx)
	if err != nil {
		data.Error = "Não foi possível carregar os paredões ativos."
	} else {
		data.Paredoes = makeVoteParedoes(paredoes)
	}

	if r.Method == http.MethodPost && data.Error == "" {
		if err := r.ParseForm(); err != nil {
			data.Error = "Não consegui ler os dados enviados. Tente novamente."
		} else {
			vote := domain.Voto{
				ParedaoID:      domain.ParedaoID(strings.TrimSpace(r.FormValue("paredao_id"))),
				ParticipanteID: domain.ParticipanteID(strings.TrimSpace(r.FormValue("participante_id"))),
				OrigemIP:       clientIP(r),
				UserAgent:      r.UserAgent(),
			}

			if vote.ParedaoID == "" || vote.ParticipanteID == "" {
				data.Error = "Selecione um participante para votar."
			} else if err := f.service.RegistrarVoto(ctx, vote); err != nil {
				data.Error = translateVoteError(err)
			} else {
				http.Redirect(w, r, "/panorama?paredao_id="+url.QueryEscape(string(vote.ParedaoID))+"&status=success", http.StatusSeeOther)
				return
			}
		}
	}

	f.render(w, "vote_body", data)
}

func (f *Frontend) handlePanorama(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	paredaoID := domain.ParedaoID(strings.TrimSpace(r.URL.Query().Get("paredao_id")))
	data := panoramaPageData{}

	if status := r.URL.Query().Get("status"); status == "success" {
		data.Message = "Voto registrado com sucesso!"
	}

	if paredaoID == "" {
		data.Error = "Informe qual paredão deseja acompanhar."
		f.render(w, "panorama_body", data)
		return
	}

	parciais, err := f.service.Parciais(ctx, paredaoID)
	if err != nil {
		data.Error = translateVoteError(err)
		f.render(w, "panorama_body", data)
		return
	}

	paredoes, err := f.service.ListarAtivos(ctx)
	if err != nil {
		// seguimos sem interromper; usaremos o ID caso não consiga o nome.
		paredoes = nil
	}

	nomeParedao, participantesNome := identifyParedao(paredoes, paredaoID)
	if nomeParedao == "" {
		nomeParedao = string(paredaoID)
	}

	data.ParedaoNome = nomeParedao
	totalGeral := int64(0)
	for _, parcial := range parciais {
		totalGeral += parcial.Total
		nome := participantesNome[parcial.ParticipanteID]
		if nome == "" {
			nome = string(parcial.ParticipanteID)
		}
		data.Participantes = append(data.Participantes, panoramaParticipanteView{
			Nome:         nome,
			TotalDisplay: displayInt(parcial.Total),
			Percent:      formatPercent(parcial.Percentual),
		})
	}
	data.TotalGeralDisplay = displayInt(totalGeral)

	if totaisHora, err := f.service.TotaisPorHora(ctx, paredaoID); err == nil {
		for _, item := range totaisHora {
			data.VotosHora = append(data.VotosHora, horaView{
				Intervalo:    formatHour(item.Hora),
				TotalDisplay: displayInt(item.Total),
			})
		}
	} else {
		data.HoraError = "Não foi possível carregar o histórico por hora."
	}

	f.render(w, "panorama_body", data)
}

func (f *Frontend) handleConsulta(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if f.consultaToken != "" && !f.isConsultaAuthorized(r) {
		data := consultaPageData{RequiresToken: true}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err == nil {
				token := r.PostFormValue("token")
				if subtle.ConstantTimeCompare([]byte(token), []byte(f.consultaToken)) == 1 {
					setConsultaAuthCookie(w)
					http.Redirect(w, r, "/consulta", http.StatusSeeOther)
					return
				}
			}
			data.TokenError = true
		}
		f.render(w, "consulta_body", data)
		return
	}

	paredoes, err := f.service.ListarAtivos(ctx)
	data := consultaPageData{}
	if err != nil {
		data.Error = "Não foi possível carregar as informações do paredão."
		f.render(w, "consulta_body", data)
		return
	}

	for _, p := range paredoes {
		parciais, err := f.service.Parciais(ctx, p.ID)
		if err != nil {
			data.Error = "Falha ao consultar as parciais do paredão."
			break
		}

		porHora, err := f.service.TotaisPorHora(ctx, p.ID)
		if err != nil {
			data.Error = "Falha ao consultar a série histórica por hora."
			break
		}

		view := consultaParedaoView{Nome: p.Nome}
		participantesNome := make(map[domain.ParticipanteID]string, len(p.Participantes))
		for _, part := range p.Participantes {
			participantesNome[part.ID] = part.Nome
		}

		totalGeral := int64(0)
		for _, parcial := range parciais {
			totalGeral += parcial.Total
			nome := participantesNome[parcial.ParticipanteID]
			if nome == "" {
				nome = string(parcial.ParticipanteID)
			}
			view.Participantes = append(view.Participantes, panoramaParticipanteView{
				Nome:         nome,
				TotalDisplay: displayInt(parcial.Total),
				Percent:      formatPercent(parcial.Percentual),
			})
		}
		view.TotalDisplay = displayInt(totalGeral)

		for _, item := range porHora {
			view.VotosHora = append(view.VotosHora, horaView{
				Intervalo:    formatHour(item.Hora),
				TotalDisplay: displayInt(item.Total),
			})
		}

		data.Paredoes = append(data.Paredoes, view)
	}

	f.render(w, "consulta_body", data)
}

func (f *Frontend) render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var content strings.Builder
	if err := f.templates.ExecuteTemplate(&content, tmpl, data); err != nil {
		http.Error(w, "erro ao montar a página", http.StatusInternalServerError)
		return
	}

	page := struct {
		Title   string
		Content template.HTML
	}{
		Title:   pageTitle(tmpl),
		Content: template.HTML(content.String()),
	}

	if err := f.templates.ExecuteTemplate(w, "layout", page); err != nil {
		http.Error(w, "erro ao renderizar página", http.StatusInternalServerError)
	}
}

func pageTitle(body string) string {
	switch body {
	case "vote_body":
		return "Votação"
	case "panorama_body":
		return "Panorama do paredão"
	case "consulta_body":
		return "Consulta de parciais"
	default:
		return "Votação BBB"
	}
}

type votePageData struct {
	Paredoes []voteParedaoView
	Error    string
}

type voteParedaoView struct {
	ID            string
	Nome          string
	Descricao     string
	Inicio        string
	Fim           string
	Participantes []voteParticipanteView
}

type voteParticipanteView struct {
	ID   string
	Nome string
}

type panoramaPageData struct {
	ParedaoNome       string
	Participantes     []panoramaParticipanteView
	TotalGeralDisplay string
	VotosHora         []horaView
	Message           string
	Error             string
	HoraError         string
}

type panoramaParticipanteView struct {
	Nome         string
	TotalDisplay string
	Percent      string
}

type horaView struct {
	Intervalo    string
	TotalDisplay string
}

type consultaPageData struct {
	RequiresToken bool
	TokenError    bool
	Error         string
	Paredoes      []consultaParedaoView
}

type consultaParedaoView struct {
	Nome          string
	TotalDisplay  string
	Participantes []panoramaParticipanteView
	VotosHora     []horaView
}

func makeVoteParedoes(paredoes []domain.Paredao) []voteParedaoView {
	views := make([]voteParedaoView, 0, len(paredoes))
	for _, p := range paredoes {
		view := voteParedaoView{
			ID:        string(p.ID),
			Nome:      p.Nome,
			Descricao: p.Descricao,
			Inicio:    formatDateTime(p.Inicio),
			Fim:       formatDateTime(p.Fim),
		}
		for _, part := range p.Participantes {
			view.Participantes = append(view.Participantes, voteParticipanteView{
				ID:   string(part.ID),
				Nome: part.Nome,
			})
		}
		views = append(views, view)
	}
	return views
}

func identifyParedao(paredoes []domain.Paredao, id domain.ParedaoID) (string, map[domain.ParticipanteID]string) {
	nomes := make(map[domain.ParticipanteID]string)
	for _, p := range paredoes {
		if p.ID == id {
			for _, part := range p.Participantes {
				nomes[part.ID] = part.Nome
			}
			return p.Nome, nomes
		}
	}
	return "", nomes
}

func translateVoteError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, antifraude.ErrRateLimitExceeded):
		return "Você atingiu o limite de votos por minuto. Aguarde um instante e tente novamente."
	case errors.Is(err, voting.ErrPeriodoEncerrado):
		return "Esse paredão já foi encerrado."
	case errors.Is(err, voting.ErrParticipanteDesconhecido):
		return "Não encontrei o participante informado."
	case errors.Is(err, voting.ErrParedaoNaoEncontrado):
		return "Paredão não encontrado."
	default:
		return "Não foi possível registrar o voto. Tente novamente."
	}
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

func displayInt(v int64) string {
	return fmt.Sprintf("%d", v)
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006 15:04")
}

func formatHour(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01 15h")
}

func (f *Frontend) isConsultaAuthorized(r *http.Request) bool {
	if f.consultaToken == "" {
		return true
	}
	cookie, err := r.Cookie("consulta-auth")
	return err == nil && cookie.Value == "ok"
}

func setConsultaAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "consulta-auth",
		Value:    "ok",
		Path:     "/consulta",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
