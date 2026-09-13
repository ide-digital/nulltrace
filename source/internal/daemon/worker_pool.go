package daemon

import (
	"context"
	"sync"
)

type Job struct {
	ActionID string
}

type Pool struct {
	jobs    chan Job
	workers int
	handler func(context.Context, Job) error
	logFn   func(string, ...any)
	wg      sync.WaitGroup
}

func NewPool(workers int, handler func(context.Context, Job) error, logFn func(string, ...any)) *Pool {
	if workers <= 0 {
		workers = 2
	}
	return &Pool{
		jobs:    make(chan Job, 64),
		workers: workers,
		handler: handler,
		logFn:   logFn,
	}
}

func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-p.jobs:
					if !ok {
						return
					}
					if err := p.handler(ctx, job); err != nil && p.logFn != nil {
						p.logFn("worker job failed", "action_id", job.ActionID, "err", err)
					}
				}
			}
		}()
	}
}

func (p *Pool) Submit(ctx context.Context, job Job) {
	select {
	case <-ctx.Done():
	case p.jobs <- job:
	}
}

func (p *Pool) Wait() { p.wg.Wait() }
