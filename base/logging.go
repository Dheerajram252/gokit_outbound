package base

import (
	"context"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/handlers"
	"time"
)

type Middleware func(Service) Service

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func NewLoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

func (mw loggingMiddleware) Check(ctx context.Context) (p bool, err error) {
	defer func(begin time.Time) {
		if err != nil {
			mw.logger.Log("method", "check", "took", time.Since(begin), "err", err)
		}

	}(time.Now())
	return mw.next.Check(ctx)
}

type panicLogger struct {
	log.Logger
}

func NewPanicLogger(logger log.Logger) handlers.RecoveryHandlerLogger {
	return panicLogger{
		logger,
	}
}

func (pl panicLogger) Println(msgs ...interface{}) {
	for _, msg := range msgs {
		pl.Log("panic", msg)
	}
}
