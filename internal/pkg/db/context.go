package db

import "context"

const ctxName = "db"

func AddToContext(ctx context.Context, conn IClient) context.Context {
	return context.WithValue(ctx, ctxName, conn)
}

func FromContext(ctx context.Context) IClient {
	conn, ok := ctx.Value(ctxName).(IClient)
	if !ok {
		panic("Not found db in context")
	}
	return conn
}
