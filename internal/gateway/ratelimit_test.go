package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeClock es un reloj que solo avanza cuando el test lo indica.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// limitedServer devuelve un servidor con 60 peticiones por minuto y ráfaga
// de 3, gobernado por un reloj falso.
func limitedServer() (*Server, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	s := NewServer(Services{})
	s.limiter = newRateLimiter(60, 3, clock.now)
	return s, clock
}

// from arma un GET cuyo origen, visto por la plataforma, es ip.
func from(ip, path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Forwarded-For", ip)
	return req
}

func TestRateLimitRejectsAfterBurst(t *testing.T) {
	s, _ := limitedServer()

	for i := range 3 {
		rec := serve(t, s, from("203.0.113.7", "/docs"))
		if rec.Code != http.StatusOK {
			t.Fatalf("petición %d: got status %d, want %d", i+1, rec.Code, http.StatusOK)
		}
		if got, want := rec.Header().Get("X-RateLimit-Remaining"), strconv.Itoa(2-i); got != want {
			t.Errorf("petición %d: got X-RateLimit-Remaining %q, want %q", i+1, got, want)
		}
	}

	rec := serve(t, s, from("203.0.113.7", "/docs"))
	p := decodeProblem(t, rec, http.StatusTooManyRequests)
	if !strings.HasSuffix(p.Type, "/rate-limited") {
		t.Errorf("got type %q, want rate-limited", p.Type)
	}
	// A una ficha por segundo, la siguiente llega en un segundo.
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("got Retry-After %q, want 1", got)
	}
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	s, clock := limitedServer()
	for range 3 {
		serve(t, s, from("203.0.113.7", "/docs"))
	}
	if rec := serve(t, s, from("203.0.113.7", "/docs")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	clock.advance(2 * time.Second)

	// Dos segundos devuelven dos fichas, no más.
	for i := range 2 {
		if rec := serve(t, s, from("203.0.113.7", "/docs")); rec.Code != http.StatusOK {
			t.Errorf("petición %d tras esperar: got status %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}
	if rec := serve(t, s, from("203.0.113.7", "/docs")); rec.Code != http.StatusTooManyRequests {
		t.Errorf("tercera tras esperar: got status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Una espera larga no acumula más que la ráfaga.
	clock.advance(time.Hour)
	for range 3 {
		serve(t, s, from("203.0.113.7", "/docs"))
	}
	if rec := serve(t, s, from("203.0.113.7", "/docs")); rec.Code != http.StatusTooManyRequests {
		t.Errorf("tras una hora: got status %d, want %d; la ráfaga no debe acumularse", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitIsPerOrigin(t *testing.T) {
	s, _ := limitedServer()
	for range 4 {
		serve(t, s, from("203.0.113.7", "/docs"))
	}

	if rec := serve(t, s, from("198.51.100.9", "/docs")); rec.Code != http.StatusOK {
		t.Errorf("otro origen: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// El cliente controla lo que pone al inicio de X-Forwarded-For; la plataforma
// agrega la dirección real al final. Cambiar el inicio en cada petición no
// debe servir para eludir el límite.
func TestRateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	s, _ := limitedServer()

	var last *httptest.ResponseRecorder
	for i := range 4 {
		last = serve(t, s, from(fmt.Sprintf("10.0.0.%d, 203.0.113.7", i), "/docs"))
	}

	if last.Code != http.StatusTooManyRequests {
		t.Errorf("got status %d, want %d: el encabezado falseado eludió el límite", last.Code, http.StatusTooManyRequests)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		remote  string
		want    string
	}{
		{"un valor", []string{"203.0.113.7"}, "10.1.1.1:5000", "203.0.113.7"},
		{"falseado más real", []string{"1.2.3.4, 203.0.113.7"}, "10.1.1.1:5000", "203.0.113.7"},
		{"varias líneas de encabezado", []string{"1.2.3.4", "5.6.7.8, 203.0.113.7"}, "10.1.1.1:5000", "203.0.113.7"},
		{"IPv6", []string{"1.2.3.4, 2001:db8::1"}, "10.1.1.1:5000", "2001:db8::1"},
		{"sin encabezado", nil, "192.0.2.10:5000", "192.0.2.10"},
		{"sin encabezado, IPv6", nil, "[2001:db8::2]:5000", "2001:db8::2"},
		{"encabezado vacío al final", []string{"1.2.3.4, "}, "192.0.2.10:5000", "192.0.2.10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, h := range tt.headers {
				req.Header.Add("X-Forwarded-For", h)
			}
			if got := clientIP(req); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// El despliegue y la plataforma consultan /health; no deben poder bloquearse.
func TestRateLimitExemptsHealth(t *testing.T) {
	s, _ := limitedServer()
	for i := range 10 {
		if rec := serve(t, s, from("203.0.113.7", "/health")); rec.Code != http.StatusOK {
			t.Fatalf("petición %d: got status %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}
	// Y no consumen el cupo del resto.
	if rec := serve(t, s, from("203.0.113.7", "/docs")); rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRateLimitAppliesToAPIAndReady(t *testing.T) {
	for _, path := range []string{"/v1/documents", "/ready", "/openapi.yaml"} {
		s, _ := limitedServer()
		var last *httptest.ResponseRecorder
		for range 4 {
			last = serve(t, s, from("203.0.113.7", path))
		}
		if last.Code != http.StatusTooManyRequests {
			t.Errorf("%s: got status %d, want %d", path, last.Code, http.StatusTooManyRequests)
		}
	}
}

func TestRateLimiterMemoryIsBounded(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	l := newRateLimiter(60, 3, clock.now)

	for i := range maxTrackedOrigins + 500 {
		l.allow(fmt.Sprintf("origen-%d", i))
	}
	if len(l.buckets) > maxTrackedOrigins {
		t.Errorf("el limitador recuerda %d orígenes, want a lo más %d", len(l.buckets), maxTrackedOrigins)
	}

	// Pasado el tiempo de recarga, los orígenes inactivos se olvidan.
	clock.advance(2 * time.Minute)
	l.allow("origen-nuevo")
	if len(l.buckets) != 1 {
		t.Errorf("tras la limpieza quedan %d orígenes, want 1", len(l.buckets))
	}
}
