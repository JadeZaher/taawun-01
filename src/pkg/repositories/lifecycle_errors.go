package repositories

import (
	"context"
	"errors"

	"github.com/mattn/go-sqlite3"
)

var ErrLifecycleUnavailable = errors.New("lifecycle transaction unavailable")

func normalizeLifecycleTransactionError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && (sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked) {
		return ErrLifecycleUnavailable
	}
	return err
}
