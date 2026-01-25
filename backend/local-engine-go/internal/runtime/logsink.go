package runtime

import "context"

type logSinkKey struct{}

func WithLogSink(ctx context.Context, sink LogSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, logSinkKey{}, sink)
}

func LogSinkFromContext(ctx context.Context) LogSink {
	return logSinkFromContext(ctx)
}

func logSinkFromContext(ctx context.Context) LogSink {
	if ctx == nil {
		return nil
	}
	if sink, ok := ctx.Value(logSinkKey{}).(LogSink); ok {
		return sink
	}
	return nil
}
