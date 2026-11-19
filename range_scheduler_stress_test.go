package hydra

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Race the scheduler against concurrent socket-read reservations. No byte
// interval may be reserved twice or omitted, even if idle workers repeatedly
// steal tails while the original owner is inside its read loop.
func TestAdaptiveSchedulerConcurrentReservationsCoverFileOnce(t *testing.T) {
	const (
		block   = int64(1 << 20)
		read    = int64(256 << 10)
		workers = 8
		size    = int64(128 << 20)
	)
	for round := 0; round < 10; round++ {
		s := newRangeScheduler(splitPieces(size, workers), block, block)
		leases := make([]*rangeLease, workers)
		for i := range leases {
			leases[i] = s.claim()
			if leases[i] == nil {
				t.Fatal("initial range was not assigned")
			}
		}

		seen := make([]atomic.Int32, size/read)
		var wg sync.WaitGroup
		failures := make(chan string, 10)
		for _, initial := range leases {
			wg.Add(1)
			go func(lease *rangeLease) {
				defer wg.Done()
				for lease != nil {
					for at := lease.job.Start; ; {
						n := lease.reserve(at, int(read))
						if n == 0 {
							break
						}
						if at%read != 0 || int64(n)%read != 0 {
							select {
							case failures <- fmt.Sprintf("unaligned reservation at=%d n=%d", at, n):
							default:
							}
							return
						}
						for pos := at; pos < at+int64(n); pos += read {
							if count := seen[pos/read].Add(1); count != 1 {
								select {
								case failures <- fmt.Sprintf("overlap at=%d count=%d", pos, count):
								default:
								}
							}
						}
						at += int64(n)
						if lease.job.Index == 0 {
							time.Sleep(150 * time.Microsecond)
						}
					}
					s.finish(lease)
					lease = s.claim()
				}
			}(initial)
		}
		wg.Wait()
		close(failures)
		for failure := range failures {
			t.Error(failure)
		}
		for index := range seen {
			if count := seen[index].Load(); count != 1 {
				t.Fatalf("round=%d block=%d reserved %d times", round, index, count)
			}
		}
		if s.nextJob <= workers {
			t.Fatalf("round=%d no slow range was ever stolen", round)
		}
	}
}
