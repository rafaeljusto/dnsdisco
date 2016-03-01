package dnsdisco

import (
	"testing"
	"time"
)

func TestLockedRandSource(t *testing.T) {
	iterations := 50
	randomNumbersCh := make(chan int, iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			randomSource.Seed(time.Now().UnixNano())
			randomNumbersCh <- randomSource.Intn(100)
		}()
	}

	for i := 0; i < iterations; i++ {
		randomNumber := <-randomNumbersCh
		if randomNumber < 0 || 100 < randomNumber {
			t.Errorf("Unexpected random number %d", randomNumber)
		}
	}
}
