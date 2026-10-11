package gateway

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Límite por origen. Una persona que explora la API desde /docs queda muy
// por debajo; un bucle que la recorre sin pausa lo alcanza en segundos.
const (
	defaultRatePerMinute = 60
	defaultRateBurst     = 30

	// maxTrackedOrigins acota la memoria del limitador. Al alcanzarlo se
	// descarta todo el estado: por un instante nadie está limitado, que es
	// preferible a rechazar orígenes legítimos o a crecer sin tope.
	maxTrackedOrigins = 10_000
)

var errRateLimited = newServiceError(
	"rate-limited",
	"Demasiadas peticiones",
	http.StatusTooManyRequests,
	"Se superó el límite de peticiones para este origen. Reintente después del plazo indicado en Retry-After.",
)

// WithRateLimit ajusta el límite de peticiones por origen: perMinute es el
// ritmo sostenido y burst, cuántas pueden llegar juntas.
func WithRateLimit(perMinute, burst int) Option {
	return func(s *Server) {
		s.limiter = newRateLimiter(perMinute, burst, time.Now)
	}
}

// rateLimiter aplica un cubo de fichas por origen. Vive en memoria: es
// exacto porque el servicio corre en una sola instancia, y se reinicia con
// ella.
type rateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	perSecond float64
	burst     float64
	now       func() time.Time
	lastSweep time.Time
}

type bucket struct {
	tokens  float64
	updated time.Time
}

func newRateLimiter(perMinute, burst int, now func() time.Time) *rateLimiter {
	return &rateLimiter{
		buckets:   map[string]*bucket{},
		perSecond: float64(perMinute) / 60,
		burst:     float64(burst),
		now:       now,
		lastSweep: now(),
	}
}

// allow consume una ficha del origen. Devuelve si la petición pasa, cuántas
// fichas le quedan y, si no pasa, cuánto debe esperar.
func (l *rateLimiter) allow(origin string) (ok bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, found := l.buckets[origin]
	if !found {
		if len(l.buckets) >= maxTrackedOrigins {
			l.buckets = map[string]*bucket{}
		}
		b = &bucket{tokens: l.burst, updated: now}
		l.buckets[origin] = b
	}

	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.updated).Seconds()*l.perSecond)
	b.updated = now

	if b.tokens < 1 {
		wait := time.Duration((1 - b.tokens) / l.perSecond * float64(time.Second))
		return false, 0, wait
	}
	b.tokens--
	return true, int(b.tokens), 0
}

// sweep elimina, a lo más una vez por minuto, los orígenes que ya
// recuperaron todas sus fichas: olvidarlos no cambia su límite.
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	refill := time.Duration(l.burst / l.perSecond * float64(time.Second))
	for origin, b := range l.buckets {
		if now.Sub(b.updated) >= refill {
			delete(l.buckets, origin)
		}
	}
}

// limit aplica el límite a todo salvo /health, que usan el despliegue y la
// plataforma y no consulta nada.
func (s *Server) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		ok, remaining, retryAfter := s.limiter.allow(clientIP(r))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			handleResponseError(w, r, errRateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP identifica el origen de una petición.
//
// En Cloud Run, el frontend de Google agrega la dirección real del cliente
// al final de X-Forwarded-For; lo que el cliente haya puesto en ese
// encabezado queda antes. Por eso se toma el último valor: el primero lo
// controla quien envía la petición, y usarlo permitiría eludir el límite
// cambiándolo en cada llamada.
//
// Sin ese encabezado, como en desarrollo local, se usa la dirección de la
// conexión.
func clientIP(r *http.Request) string {
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		last := values[len(values)-1]
		if ip := strings.TrimSpace(last[strings.LastIndex(last, ",")+1:]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
