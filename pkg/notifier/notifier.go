package notifier

import (
	"context"

	task "scheduler/internal/task"
)

type Sender interface {
	SendAlert(ctx context.Context, t task.Task, errMsg string) error
}
