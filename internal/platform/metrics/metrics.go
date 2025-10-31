package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	voteRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bbb_vote_requests_total",
		Help: "Total de requisicoes de voto recebidas",
	}, []string{"status"})

	voteProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bbb_vote_processed_total",
		Help: "Total de votos processados pelo worker",
	})

	voteProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "bbb_vote_processing_duration_seconds",
		Help:    "Tempo para processar um voto no worker",
		Buckets: prometheus.DefBuckets,
	})
)

func ObserveVoteRequest(status string) {
	voteRequestsTotal.WithLabelValues(status).Inc()
}

func IncVoteProcessed() {
	voteProcessedTotal.Inc()
}

func ObserveProcessingDuration(seconds float64) {
	voteProcessingDuration.Observe(seconds)
}
