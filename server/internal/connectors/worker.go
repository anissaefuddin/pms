package connectors

import (
	"context"
	"time"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/riverqueue/river"
)

type Args struct{}

func (Args) Kind() string { return "connector_sync" }

type Worker struct {
	river.WorkerDefaults[Args]
	Svc *Service
}

func (w *Worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	w.Svc.RunDue(ctx)
	return nil
}

// Option registers the polling job on the shared River client (see mail.New): every minute it syncs the installs that are due.
func Option(svc *Service) mail.Option {
	return func(workers *river.Workers, periodic *[]*river.PeriodicJob) {
		river.AddWorker(workers, &Worker{Svc: svc})
		*periodic = append(*periodic, river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return Args{}, &river.InsertOpts{MaxAttempts: 1} }, nil))
	}
}
