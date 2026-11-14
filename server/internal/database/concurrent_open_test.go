package database_test

import (
	"context"
	"sync"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/database"
	"github.com/anissaefuddin/pms/server/internal/testdb"
)

// DR-05: two processes opening an empty database at once (app start and `pms bootstrap`) must both succeed.
func TestOpenConcurrentlyOnEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	url := testdb.New(t)
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pool, err := database.Open(ctx, url)
			if err == nil {
				defer pool.Close()
				err = database.Ready(ctx, pool)
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("opener %d: %v", i, err)
		}
	}
}
