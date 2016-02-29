package dnsdisco

import (
	"math/rand"
	"sync"
	"time"
)

// randomSource generate random numbers for all necessary places inside this
// library.
var randomSource *rand.Rand

func init() {
	randomSource = rand.New(&lockedRandSource{
		Source: rand.NewSource(time.Now().UnixNano()),
	})
}

// lockedRandSource prevent concurrent use of the underlying source. This
// approach was a recommendation [1] of Nishanth Shanmugham, from Google.
//
// [1] http://nishanths.svbtle.com/do-not-seed-the-global-random
type lockedRandSource struct {
	sync.Mutex
	rand.Source
}

// Int63 satisfy rand.Source interface.
func (r *lockedRandSource) Int63() int64 {
	r.Lock()
	defer r.Unlock()
	return r.Source.Int63()
}

// Seed satisfy rand.Source interface.
func (r *lockedRandSource) Seed(seed int64) {
	r.Lock()
	defer r.Unlock()
	r.Source.Seed(seed)
}
