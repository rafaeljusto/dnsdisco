package dnsdisco

import (
	"math/rand"
	"sync"
	"time"
)

var randomSource *rand.Rand

func init() {
	randomSource = rand.New(&lockedRandSource{
		src: rand.NewSource(time.Now().UnixNano()),
	})
}

// lockedRandSource prevent concurrent use of the underlying source. This
// approach was a recommendation [1] of Nishanth Shanmugham, from Google.
//
// [1] http://nishanths.svbtle.com/do-not-seed-the-global-random
type lockedRandSource struct {
	lock sync.Mutex // protects src
	src  rand.Source
}

// Int63 satisfy rand.Source interface.
func (r *lockedRandSource) Int63() int64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	ret := r.src.Int63()
	return ret
}

// Seed satisfy rand.Source interface.
func (r *lockedRandSource) Seed(seed int64) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.src.Seed(seed)
}
