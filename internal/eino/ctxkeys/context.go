package ctxkeys

import (
	"context"

	"github.com/Colin4k1024/hermesx/internal/tools"
)

type toolCtxKey struct{}

func WithToolContext(ctx context.Context, tctx *tools.ToolContext) context.Context {
	return context.WithValue(ctx, toolCtxKey{}, tctx)
}

func ToolContextFrom(ctx context.Context) *tools.ToolContext {
	v, _ := ctx.Value(toolCtxKey{}).(*tools.ToolContext)
	return v
}
