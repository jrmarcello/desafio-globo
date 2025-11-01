package voting

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/marcelojr/desafio-globo/internal/domain"
	"github.com/marcelojr/desafio-globo/internal/platform/ids"
)

func TestServiceCriarParedao(t *testing.T) {
	deps := newServiceDeps()
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	inicio := deps.baseTime
	fim := deps.baseTime.Add(2 * time.Hour)

	paredao, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:      "Paredão Teste",
		Descricao: "Primeiro paredão",
		Inicio:    inicio,
		Fim:       fim,
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("esperava criar paredao sem erro, mas veio: %v", err)
	}

	if paredao.ID == "" {
		t.Fatal("ID não pode ser vazio")
	}
	if len(paredao.Participantes) != 2 {
		t.Fatalf("esperava 2 participantes, veio %d", len(paredao.Participantes))
	}

	got, err := deps.paredaoRepo.FindByID(context.Background(), paredao.ID)
	if err != nil {
		t.Fatalf("falha ao buscar paredao salvo: %v", err)
	}
	if got.Nome != "Paredão Teste" {
		t.Fatalf("nome salvo incorreto, esperado %q, veio %q", "Paredão Teste", got.Nome)
	}
}

func TestServiceRegistrarVotoEnfileira(t *testing.T) {
	deps := newServiceDeps()
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	paredao, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:      "Paredão",
		Inicio:    deps.baseTime,
		Fim:       deps.baseTime.Add(3 * time.Hour),
		Descricao: "Teste",
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("falha ao criar paredao: %v", err)
	}

	err = service.RegistrarVoto(context.Background(), domain.Voto{
		ParedaoID:      paredao.Participantes[0].ParedaoID,
		ParticipanteID: paredao.Participantes[0].ID,
		OrigemIP:       "127.0.0.1",
		UserAgent:      "teste",
	})
	if err != nil {
		t.Fatalf("esperava registrar voto sem erro, mas veio: %v", err)
	}

	if deps.queue.Len() != 1 {
		t.Fatalf("voto deveria ter sido enfileirado; total esperado 1, veio %d", deps.queue.Len())
	}
	if len(deps.votoRepo.lista) != 0 {
		t.Fatalf("voto não deveria ter sido persistido antes do worker, total persistido %d", len(deps.votoRepo.lista))
	}
}

func TestServiceParciaisComWorker(t *testing.T) {
	deps := newServiceDeps()
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	paredao, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:   "Paredão",
		Inicio: deps.baseTime,
		Fim:    deps.baseTime.Add(2 * time.Hour),
		Ativo:  true,
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("erro criando paredao: %v", err)
	}

	voto := domain.Voto{
		ParedaoID:      paredao.ID,
		ParticipanteID: paredao.Participantes[0].ID,
		OrigemIP:       "127.0.0.1",
		UserAgent:      "teste",
	}
	if err := service.RegistrarVoto(context.Background(), voto); err != nil {
		t.Fatalf("erro registrando voto: %v", err)
	}
	voto2 := domain.Voto{
		ParedaoID:      paredao.ID,
		ParticipanteID: paredao.Participantes[1].ID,
		OrigemIP:       "127.0.0.2",
		UserAgent:      "teste",
	}
	if err := service.RegistrarVoto(context.Background(), voto2); err != nil {
		t.Fatalf("erro registrando segundo voto: %v", err)
	}

	for _, votoEnfileirado := range deps.queue.Drain() {
		if err := deps.votoRepo.Registrar(context.Background(), votoEnfileirado); err != nil {
			t.Fatalf("erro persistindo voto: %v", err)
		}
		if _, err := deps.contador.Incrementar(context.Background(), CounterKeyTotalParedao(votoEnfileirado.ParedaoID), 1); err != nil {
			t.Fatalf("erro incrementando contador total: %v", err)
		}
		if _, err := deps.contador.Incrementar(context.Background(), CounterKeyParticipante(votoEnfileirado.ParedaoID, votoEnfileirado.ParticipanteID), 1); err != nil {
			t.Fatalf("erro incrementando contador do participante: %v", err)
		}
	}

	parciais, err := service.Parciais(context.Background(), paredao.ID)
	if err != nil {
		t.Fatalf("erro obtendo parciais: %v", err)
	}
	if len(parciais) != 2 {
		t.Fatalf("esperava 2 parciais, veio %d", len(parciais))
	}

	var total int64
	for _, parcial := range parciais {
		total += parcial.Total
	}
	if total != 2 {
		t.Fatalf("total de votos deveria ser 2, veio %d", total)
	}
}

type serviceDependencies struct {
	paredaoRepo      *inMemoryParedaoRepo
	participanteRepo *inMemoryParticipanteRepo
	votoRepo         *inMemoryVotoRepo
	contador         *inMemoryContador
	queue            *recordingQueue
	antifraude       domain.Antifraude
	clock            *staticClock
	idGen            *ids.Generator
	baseTime         time.Time
}

func newServiceDeps() serviceDependencies {
	base := time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC)

	return serviceDependencies{
		paredaoRepo:      newInMemoryParedaoRepo(),
		participanteRepo: newInMemoryParticipanteRepo(),
		votoRepo:         newInMemoryVotoRepo(),
		contador:         newInMemoryContador(),
		queue:            newRecordingQueue(),
		antifraude:       antifraudeNoop{},
		clock:            &staticClock{now: base},
		idGen:            ids.NewGenerator(),
		baseTime:         base,
	}
}

type inMemoryParedaoRepo struct {
	mu   sync.Mutex
	data map[domain.ParedaoID]domain.Paredao
}

func newInMemoryParedaoRepo() *inMemoryParedaoRepo {
	return &inMemoryParedaoRepo{data: make(map[domain.ParedaoID]domain.Paredao)}
}

func (r *inMemoryParedaoRepo) Create(_ context.Context, p domain.Paredao) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[p.ID] = p
	return nil
}

func (r *inMemoryParedaoRepo) Update(_ context.Context, p domain.Paredao) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[p.ID]; !ok {
		return domain.ErrNotFound
	}
	r.data[p.ID] = p
	return nil
}

func (r *inMemoryParedaoRepo) FindByID(_ context.Context, id domain.ParedaoID) (domain.Paredao, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.data[id]
	if !ok {
		return domain.Paredao{}, domain.ErrNotFound
	}
	return p, nil
}

func (r *inMemoryParedaoRepo) ListAtivos(_ context.Context) ([]domain.Paredao, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domain.Paredao
	for _, p := range r.data {
		if p.Ativo {
			result = append(result, p)
		}
	}
	return result, nil
}

type inMemoryParticipanteRepo struct {
	mu        sync.Mutex
	porParedo map[domain.ParedaoID][]domain.Participante
}

func newInMemoryParticipanteRepo() *inMemoryParticipanteRepo {
	return &inMemoryParticipanteRepo{porParedo: make(map[domain.ParedaoID][]domain.Participante)}
}

func (r *inMemoryParticipanteRepo) BulkCreate(_ context.Context, paredaoID domain.ParedaoID, participantes []domain.Participante) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.porParedo[paredaoID] = append([]domain.Participante(nil), participantes...)
	return nil
}

func (r *inMemoryParticipanteRepo) ListByParedao(_ context.Context, paredaoID domain.ParedaoID) ([]domain.Participante, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	participantes := r.porParedo[paredaoID]
	copia := make([]domain.Participante, len(participantes))
	copy(copia, participantes)
	return copia, nil
}

type inMemoryVotoRepo struct {
	mu    sync.Mutex
	lista []domain.Voto
}

func newInMemoryVotoRepo() *inMemoryVotoRepo {
	return &inMemoryVotoRepo{}
}

func (r *inMemoryVotoRepo) Registrar(_ context.Context, voto domain.Voto) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lista = append(r.lista, voto)
	return nil
}

func (r *inMemoryVotoRepo) TotalPorParedao(_ context.Context, id domain.ParedaoID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var total int64
	for _, voto := range r.lista {
		if voto.ParedaoID == id {
			total++
		}
	}
	return total, nil
}

func (r *inMemoryVotoRepo) TotalPorParticipante(_ context.Context, paredaoID domain.ParedaoID) (map[domain.ParticipanteID]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[domain.ParticipanteID]int64)
	for _, voto := range r.lista {
		if voto.ParedaoID == paredaoID {
			result[voto.ParticipanteID]++
		}
	}
	return result, nil
}

func (r *inMemoryVotoRepo) TotalPorHora(_ context.Context, paredaoID domain.ParedaoID) ([]domain.ParcialHora, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	porHora := make(map[time.Time]int64)
	for _, voto := range r.lista {
		if voto.ParedaoID != paredaoID {
			continue
		}
		hora := voto.CriadoEm.Truncate(time.Hour)
		porHora[hora]++
	}
	var resultado []domain.ParcialHora
	for hora, total := range porHora {
		resultado = append(resultado, domain.ParcialHora{
			ParedaoID: paredaoID,
			Hora:      hora,
			Total:     total,
		})
	}
	return resultado, nil
}

type inMemoryContador struct {
	mu      sync.Mutex
	valores map[string]int64
}

func newInMemoryContador() *inMemoryContador {
	return &inMemoryContador{valores: make(map[string]int64)}
}

func (c *inMemoryContador) Incrementar(_ context.Context, chave string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.valores[chave] += delta
	return c.valores[chave], nil
}

func (c *inMemoryContador) Obter(_ context.Context, chave string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.valores[chave], nil
}

func (c *inMemoryContador) ObterTodos(_ context.Context, chaves []string) (map[string]int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]int64)
	for _, chave := range chaves {
		result[chave] = c.valores[chave]
	}
	return result, nil
}

type recordingQueue struct {
	mu    sync.Mutex
	votos []domain.Voto
}

func newRecordingQueue() *recordingQueue {
	return &recordingQueue{}
}

func (r *recordingQueue) PublicarVoto(_ context.Context, voto domain.Voto) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.votos = append(r.votos, voto)
	return nil
}

func (r *recordingQueue) ConsumirVotos(ctx context.Context, handler func(context.Context, domain.Voto) error) error {
	for _, voto := range r.Drain() {
		if err := handler(ctx, voto); err != nil {
			return err
		}
	}
	return nil
}

func (r *recordingQueue) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.votos)
}

func (r *recordingQueue) Drain() []domain.Voto {
	r.mu.Lock()
	defer r.mu.Unlock()
	copia := make([]domain.Voto, len(r.votos))
	copy(copia, r.votos)
	r.votos = nil
	return copia
}

type antifraudeNoop struct{}

func (antifraudeNoop) Validar(_ context.Context, _ domain.Voto) error { return nil }

type staticClock struct {
	now time.Time
}

func (s *staticClock) Agora() time.Time {
	return s.now
}

// TestServiceListarAtivos testa listagem de paredões ativos
func TestServiceListarAtivos(t *testing.T) {
	deps := newServiceDeps() // Novo deps = novo repositório limpo
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	// Cria paredão ativo
	paredaoAtivo, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:   "Paredão Ativo",
		Inicio: deps.baseTime,
		Fim:    deps.baseTime.Add(2 * time.Hour),
		Ativo:  true,
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("erro criando paredao ativo: %v", err)
	}

	// Cria paredão inativo
	_, err = service.CriarParedao(context.Background(), domain.Paredao{
		Nome:   "Paredão Inativo",
		Inicio: deps.baseTime,
		Fim:    deps.baseTime.Add(2 * time.Hour),
		Ativo:  false,
	}, []domain.Participante{
		{Nome: "Carlos"},
		{Nome: "Diana"},
	})
	if err != nil {
		t.Fatalf("erro criando paredao inativo: %v", err)
	}

	// Lista apenas ativos
	ativos, err := service.ListarAtivos(context.Background())
	if err != nil {
		t.Fatalf("erro listando ativos: %v", err)
	}

	if len(ativos) < 1 {
		t.Fatalf("esperava ao menos 1 paredao ativo, veio %d", len(ativos))
	}

	// Verifica que encontrou o paredão que criamos
	found := false
	for _, p := range ativos {
		if p.ID == paredaoAtivo.ID {
			found = true
			// Verifica que o paredão ativo tem os participantes carregados
			if len(p.Participantes) != 2 {
				t.Errorf("esperava 2 participantes no paredão ativo, veio %d", len(p.Participantes))
			}
			break
		}
	}

	if !found {
		t.Error("paredao ativo criado não foi encontrado na listagem")
	}
}

// TestServiceTotaisPorHora testa agregação de votos por hora
func TestServiceTotaisPorHora(t *testing.T) {
	deps := newServiceDeps()
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	paredao, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:   "Paredão Teste",
		Inicio: deps.baseTime,
		Fim:    deps.baseTime.Add(24 * time.Hour),
		Ativo:  true,
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("erro criando paredao: %v", err)
	}

	// Registra votos
	for i := 0; i < 3; i++ {
		voto := domain.Voto{
			ParedaoID:      paredao.ID,
			ParticipanteID: paredao.Participantes[0].ID,
			OrigemIP:       "127.0.0.1",
			UserAgent:      "test",
		}
		if err := service.RegistrarVoto(context.Background(), voto); err != nil {
			t.Fatalf("erro registrando voto: %v", err)
		}
	}

	// Persiste votos da fila
	for _, v := range deps.queue.Drain() {
		if err := deps.votoRepo.Registrar(context.Background(), v); err != nil {
			t.Fatalf("erro persistindo voto: %v", err)
		}
	}

	// Busca totais por hora
	totais, err := service.TotaisPorHora(context.Background(), paredao.ID)
	if err != nil {
		t.Fatalf("erro buscando totais por hora: %v", err)
	}

	if len(totais) == 0 {
		t.Error("esperava ao menos uma entrada de total por hora")
	}
}

// TestServiceRegistrarVotoComValidacoes testa validações de voto
func TestServiceRegistrarVotoComValidacoes(t *testing.T) {
	deps := newServiceDeps()
	service := NewService(
		deps.paredaoRepo,
		deps.participanteRepo,
		deps.votoRepo,
		deps.contador,
		deps.queue,
		deps.antifraude,
		deps.clock,
		deps.idGen,
	)

	paredao, err := service.CriarParedao(context.Background(), domain.Paredao{
		Nome:   "Paredão",
		Inicio: deps.baseTime.Add(-1 * time.Hour),
		Fim:    deps.baseTime.Add(1 * time.Hour),
		Ativo:  true,
	}, []domain.Participante{
		{Nome: "Alice"},
		{Nome: "Bruno"},
	})
	if err != nil {
		t.Fatalf("erro criando paredao: %v", err)
	}

	tests := []struct {
		name    string
		voto    domain.Voto
		wantErr bool
	}{
		{
			name: "voto válido",
			voto: domain.Voto{
				ParedaoID:      paredao.ID,
				ParticipanteID: paredao.Participantes[0].ID,
				OrigemIP:       "127.0.0.1",
				UserAgent:      "test",
			},
			wantErr: false,
		},
		{
			name: "paredão inexistente",
			voto: domain.Voto{
				ParedaoID:      domain.ParedaoID("inexistente"),
				ParticipanteID: paredao.Participantes[0].ID,
				OrigemIP:       "127.0.0.1",
				UserAgent:      "test",
			},
			wantErr: true,
		},
		{
			name: "participante inexistente",
			voto: domain.Voto{
				ParedaoID:      paredao.ID,
				ParticipanteID: domain.ParticipanteID("inexistente"),
				OrigemIP:       "127.0.0.1",
				UserAgent:      "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RegistrarVoto(context.Background(), tt.voto)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegistrarVoto() erro = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
