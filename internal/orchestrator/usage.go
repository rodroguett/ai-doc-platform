package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// defaultUsageDays es el periodo que se informa cuando no se pide uno.
	defaultUsageDays = 30

	// maxUsageDays acota el periodo de un informe de uso: un año, más el
	// día bisiesto.
	maxUsageDays = 366

	day = 24 * time.Hour
)

// UsagePeriod es el periodo de un informe de uso, en días UTC, con ambos
// extremos incluidos. Un extremo en cero toma su valor por omisión.
type UsagePeriod struct {
	From, To time.Time
}

// UsageSummary es el consumo agregado de un periodo (ADR-0013).
type UsageSummary struct {
	// From y To son el primer y el último día incluidos.
	From, To time.Time

	Queries     int
	CacheHits   int
	TokensTotal int64

	// EstimatedCostUSD es lo que habrían costado las consultas a tarifa de
	// lista; no es lo que se paga. Excluye las consultas sin costo conocido,
	// que cuenta QueriesWithoutCost.
	EstimatedCostUSD   float64
	QueriesWithoutCost int

	ByProvider []ProviderUsage
}

// ProviderUsage es el consumo atribuido a un proveedor.
type ProviderUsage struct {
	Provider         string
	Queries          int
	EstimatedCostUSD float64
}

// UsageReader suma el registro de uso entre since (incluido) y until
// (excluido). Devuelve el resumen sin las fechas del periodo.
type UsageReader interface {
	Summarize(ctx context.Context, since, until time.Time) (UsageSummary, error)

	// CountNew cuenta las consultas registradas desde since que no vinieron
	// de la caché: las que gastaron una petición de embeddings.
	CountNew(ctx context.Context, since time.Time) (int, error)
}

// UsageStore guarda y lee el registro de uso.
type UsageStore interface {
	UsageRecorder
	UsageReader
}

// Usage informa el consumo de un periodo.
func (s *Service) Usage(ctx context.Context, p UsagePeriod) (UsageSummary, error) {
	if s.usage == nil {
		return UsageSummary{}, fmt.Errorf("%w: el registro de uso no está configurado", ErrUnavailable)
	}
	from, to, err := p.normalize(s.now())
	if err != nil {
		return UsageSummary{}, err
	}

	summary, err := s.usage.Summarize(ctx, from, to.Add(day))
	if err != nil {
		if ctx.Err() != nil {
			return UsageSummary{}, ctx.Err()
		}
		if errors.Is(err, ErrUnavailable) {
			return UsageSummary{}, err
		}
		return UsageSummary{}, fmt.Errorf("%w: no se pudo leer el registro de uso", ErrUnavailable)
	}
	summary.From, summary.To = from, to
	return summary, nil
}

// normalize aplica los valores por omisión y valida el periodo. Devuelve el
// primer y el último día, a medianoche UTC.
func (p UsagePeriod) normalize(now time.Time) (time.Time, time.Time, error) {
	to := midnight(now)
	if !p.To.IsZero() {
		to = midnight(p.To)
	}
	from := to.Add(-(defaultUsageDays - 1) * day)
	if !p.From.IsZero() {
		from = midnight(p.From)
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: from no puede ser posterior a to", ErrInvalidInput)
	}
	if days := int(to.Sub(from)/day) + 1; days > maxUsageDays {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: el periodo no puede superar %d días", ErrInvalidInput, maxUsageDays)
	}
	return from, to, nil
}

func midnight(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
