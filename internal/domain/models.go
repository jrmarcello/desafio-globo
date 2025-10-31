package domain

import (
	"time"
)

type (
	ParedaoID      string
	ParticipanteID string
	VotoID         string
)

type Paredao struct {
	ID            ParedaoID      `gorm:"column:id;type:char(26);primaryKey"`
	Nome          string         `gorm:"column:nome;type:text;not null"`
	Descricao     string         `gorm:"column:descricao;type:text"`
	Inicio        time.Time      `gorm:"column:inicio;not null"`
	Fim           time.Time      `gorm:"column:fim;not null"`
	Participantes []Participante `gorm:"foreignKey:ParedaoID;constraint:OnDelete:CASCADE"`
	Ativo         bool           `gorm:"column:ativo;not null;default:true"`
	CriadoEm      time.Time      `gorm:"column:criado_em;autoCreateTime"`
	AtualizadoEm  time.Time      `gorm:"column:atualizado_em;autoUpdateTime"`
}

type Participante struct {
	ID           ParticipanteID `gorm:"column:id;type:char(26);primaryKey"`
	ParedaoID    ParedaoID      `gorm:"column:paredao_id;type:char(26);not null;index"`
	Nome         string         `gorm:"column:nome;type:text;not null"`
	FotoURL      string         `gorm:"column:foto_url;type:text"`
	CriadoEm     time.Time      `gorm:"column:criado_em;autoCreateTime"`
	AtualizadoEm time.Time      `gorm:"column:atualizado_em;autoUpdateTime"`
}

type Voto struct {
	ID             VotoID         `gorm:"column:id;type:char(26);primaryKey"`
	ParedaoID      ParedaoID      `gorm:"column:paredao_id;type:char(26);not null;index:idx_votos_paredao;index:idx_votos_paredao_criado_em,priority:1"`
	ParticipanteID ParticipanteID `gorm:"column:participante_id;type:char(26);not null;index:idx_votos_participante"`
	OrigemIP       string         `gorm:"column:origem_ip;type:inet"`
	UserAgent      string         `gorm:"column:user_agent;type:text"`
	CriadoEm       time.Time      `gorm:"column:criado_em;autoCreateTime;index:idx_votos_paredao_criado_em,priority:2"`
}

type Parcial struct {
	ParedaoID      ParedaoID
	ParticipanteID ParticipanteID
	Total          int64
	Percentual     float64
}

type ParcialHora struct {
	ParedaoID ParedaoID
	Hora      time.Time
	Total     int64
}

func (Paredao) TableName() string { return "paredoes" }

func (Participante) TableName() string { return "participantes" }

func (Voto) TableName() string { return "votos" }
