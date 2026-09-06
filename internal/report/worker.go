package report

import (
	"context"
	"time"
)

type jobRepository interface {
	ClaimAIJob(context.Context) (AIJob, bool, error)
	CompleteAIJob(context.Context, string, string) error
	RetryOrFailAIJob(context.Context, AIJob) error
}

type Worker struct {
	repository jobRepository
	generator  Generator
}

func NewWorker(repository jobRepository, generator Generator) *Worker {
	return &Worker{repository: repository, generator: generator}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		worked, err := w.ProcessOne(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil || !worked {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	job, found, err := w.repository.ClaimAIJob(ctx)
	if err != nil || !found {
		return false, err
	}
	if w.generator == nil {
		return true, w.repository.RetryOrFailAIJob(ctx, AIJob{ID: job.ID, Attempts: job.MaxAttempts, MaxAttempts: job.MaxAttempts})
	}
	generationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	content, generationErr := w.generator.Generate(generationCtx, job.Prompt)
	cancel()
	if generationErr != nil {
		return true, w.repository.RetryOrFailAIJob(ctx, job)
	}
	return true, w.repository.CompleteAIJob(ctx, job.ID, content)
}
