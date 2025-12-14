package main

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

func FindGitReposParallel(root string, workers int) ([]string, error) {
	type job struct{ path string }

	var wg sync.WaitGroup
	var results []string
	var resultsMu sync.Mutex

	jobs := make(chan job, 4096)

	active := atomic.Int64{}

	// Worker function
	worker := func() {
		defer wg.Done()

		for j := range jobs {
			entries, err := os.ReadDir(j.path)
			if err != nil {
				if active.Add(-1) == 0 {
					// Nobody left → close channel
					close(jobs)
				}
				continue
			}

			// Look for .git
			var found bool
			for _, e := range entries {
				if e.IsDir() && e.Name() == ".git" {
					resultsMu.Lock()
					results = append(results, j.path)
					resultsMu.Unlock()
					found = true
					break
				}
			}

			// Prune at repo root
			if !found {
				for _, e := range entries {
					if e.IsDir() {
						active.Add(1)
						jobs <- job{filepath.Join(j.path, e.Name())}
					}
				}
			}

			// Mark this job done
			if active.Add(-1) == 0 {
				close(jobs)
			}
		}
	}

	// Start workers
	wg.Add(workers)
	for range workers {
		go worker()
	}

	// Seed root
	jobs <- job{root}

	// Wait for workers
	wg.Wait()

	return results, nil
}
