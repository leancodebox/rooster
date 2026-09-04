package store

import (
	"context"

	"github.com/leancodebox/rooster/internal/domain"
)

var ErrNotFound = domainError("not found")

type domainError string

func (e domainError) Error() string { return string(e) }

type Store interface {
	Close() error
	ListTasks(context.Context) ([]domain.Task, error)
	GetTask(context.Context, string) (domain.Task, error)
	CreateTask(context.Context, domain.Task) (domain.Task, error)
	UpdateTask(context.Context, domain.Task) (domain.Task, error)
	DeleteTask(context.Context, string) error
	SetTaskEnabled(context.Context, string, bool) error
	CreateExecution(context.Context, domain.Execution) (domain.Execution, error)
	GetExecution(context.Context, string) (domain.Execution, error)
	UpdateExecution(context.Context, domain.Execution) error
	ListExecutions(context.Context, string, int) ([]domain.Execution, error)
	MarkInterruptedExecutions(context.Context) error
}
