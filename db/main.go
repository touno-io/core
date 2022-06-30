package db

import "github.com/getsentry/sentry-go"

const (
	ENV        = "ENV"
	DEBUG      = "DEBUG"
	SENTRY_ENV = "SENTRY_ENV"
	SENTRY_DSN = "SENTRY_DSN"
)

func IsRollback(err error, stx *PGTx) bool {
	if err != nil && !stx.Closed {
		stx.Rollback()
	}
	return err != nil
}

func IsRollbackThrow(err error, stx *PGTx) bool {
	if err != nil {
		Error(err)
		sentry.CaptureException(err)
		if stx != nil && !stx.Closed {
			stx.Rollback()
		}
	}
	return err != nil
}
