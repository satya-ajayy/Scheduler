package notifications

import (
	// Go Internal Packages
	"context"

	// Local Packages
	models "scheduler/models"
)

type Sender interface {
	SendAlert(ctx context.Context, t models.Task, errMsg string) error
}
