package ids

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

type Generator struct {
	mu      sync.Mutex
	entropy *ulid.MonotonicEntropy
}

func NewGenerator() *Generator {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &Generator{
		entropy: ulid.Monotonic(src, 0),
	}
}

func (g *Generator) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now().UTC()), g.entropy).String()
}

var (
	defaultOnce sync.Once
	defaultGen  *Generator
)

func DefaultGenerator() *Generator {
	defaultOnce.Do(func() {
		defaultGen = NewGenerator()
	})
	return defaultGen
}

func NewULID() string {
	return DefaultGenerator().New()
}
