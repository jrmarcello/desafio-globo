package clock

import "time"

type SystemClock struct{}

func NewSystemClock() SystemClock {
	return SystemClock{}
}

func (SystemClock) Agora() time.Time {
	return time.Now().UTC()
}
