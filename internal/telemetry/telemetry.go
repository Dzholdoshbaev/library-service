package telemetry

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm/logger"
)

type Telemetry struct {
	Registry *prometheus.Registry
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	DBTime   prometheus.Histogram
	DBErrors prometheus.Counter
	Tracer   trace.Tracer
	provider *sdktrace.TracerProvider
}

func New(ctx context.Context, endpoint string) (*Telemetry, error) {
	r := prometheus.NewRegistry()
	t := &Telemetry{Registry: r, Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "library_http_requests_total", Help: "HTTP requests by bounded route, method and status."}, []string{"route", "method", "status"}), Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "library_http_duration_seconds", Help: "HTTP request duration."}, []string{"route", "method"}), DBTime: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "library_db_operation_seconds", Help: "GORM operation duration; no statements or values recorded."}), DBErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "library_db_errors_total", Help: "Failed GORM operations."})}
	r.MustRegister(t.Requests, t.Duration, t.DBTime, t.DBErrors, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", "library-service"))), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1)))}
	if endpoint != "" {
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, err
		}
		options = append(options, sdktrace.WithBatcher(exp))
	} else {
		options = append(options, sdktrace.WithSampler(sdktrace.NeverSample()))
	}
	t.provider = sdktrace.NewTracerProvider(options...)
	t.Tracer = t.provider.Tracer("library-service/http")
	return t, nil
}
func (t *Telemetry) Shutdown(ctx context.Context) error { return t.provider.Shutdown(ctx) }
func (t *Telemetry) Pool(pool *sql.DB) {
	t.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "library_db_open_connections", Help: "Open PostgreSQL pool connections."}, func() float64 { return float64(pool.Stats().OpenConnections) }), prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "library_db_in_use_connections", Help: "In-use PostgreSQL pool connections."}, func() float64 { return float64(pool.Stats().InUse) }))
}

// SQLLogger intentionally never evaluates the SQL formatter: it can contain secrets.
type SQLLogger struct{ Telemetry *Telemetry }

func (l SQLLogger) LogMode(logger.LogLevel) logger.Interface { return l }
func (SQLLogger) Info(context.Context, string, ...any)       {}
func (SQLLogger) Warn(context.Context, string, ...any)       {}
func (SQLLogger) Error(context.Context, string, ...any)      {}
func (l SQLLogger) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	l.Telemetry.DBTime.Observe(time.Since(begin).Seconds())
	if err != nil {
		l.Telemetry.DBErrors.Inc()
	}
}
