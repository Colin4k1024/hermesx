package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var pgxQueryDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "hermes_pgx_query_duration_seconds",
		Help:    "PostgreSQL query latency via pgx tracer.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	},
	[]string{"sql_prefix"},
)

var pgxTracer = otel.Tracer("hermes-pgx")

type pgxTracerStartKey struct{}

type pgxStartData struct {
	start     time.Time
	sqlPrefix string
	span      trace.Span
}

// PGXTracer implements pgx.QueryTracer for distributed tracing and slow-query logging.
type PGXTracer struct{}

func (t *PGXTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	prefix := sqlPrefix(data.SQL)
	ctx, span := pgxTracer.Start(ctx, "store.Query",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", prefix),
		),
	)
	return context.WithValue(ctx, pgxTracerStartKey{}, &pgxStartData{
		start:     time.Now(),
		sqlPrefix: prefix,
		span:      span,
	})
}

func (t *PGXTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	sd, ok := ctx.Value(pgxTracerStartKey{}).(*pgxStartData)
	if !ok || sd == nil {
		return
	}
	elapsed := time.Since(sd.start)
	pgxQueryDuration.WithLabelValues(sd.sqlPrefix).Observe(elapsed.Seconds())

	if data.Err != nil {
		sd.span.RecordError(data.Err)
		sd.span.SetStatus(codes.Error, data.Err.Error())
	}
	sd.span.SetAttributes(attribute.Int64("db.duration_ms", elapsed.Milliseconds()))
	sd.span.End()

	if elapsed > 500*time.Millisecond {
		ContextLogger(ctx).Warn("slow_query",
			"sql_prefix", sd.sqlPrefix,
			"latency_ms", elapsed.Milliseconds(),
			"err", data.Err,
		)
	}
}

func sqlPrefix(sql string) string {
	if len(sql) > 40 {
		return sql[:40]
	}
	return sql
}

var _ pgx.QueryTracer = (*PGXTracer)(nil)
