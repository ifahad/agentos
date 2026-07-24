package server

import (
	"net/http"
	"strconv"
	"time"
)

// maxBackoff caps a single inter-attempt wait. Callers must never sleep longer
// than this even if the provider asks for more, so one upstream cannot pin a
// gateway goroutine.
const maxBackoff = 8 * time.Second

// baseBackoff is the first retry's delay before jitter.
const baseBackoff = 250 * time.Millisecond

// retryable reports whether an upstream outcome is worth another attempt.
// Transport errors, 429, and 5xx are transient; every other 4xx is the caller's
// fault and retrying it only wastes budget.
func retryable(status int, err error) bool {
	if err != nil {
		return true
	}
	if status == http.StatusTooManyRequests {
		return true
	}
	return status >= 500
}

// parseRetryAfter reads the delay-seconds form of a Retry-After header, clamped
// to maxBackoff. The HTTP-date form is not honored: providers use the seconds
// form, and a bad date must not translate into a long sleep.
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		return 0, false
	}
	d := time.Duration(secs) * time.Second
	if d > maxBackoff {
		d = maxBackoff
	}
	return d, true
}

// backoffDelay returns how long to wait before the next attempt: an explicit
// Retry-After when the provider sent one, else exponential backoff with jitter.
// rnd returns a value in [0,1) and is injected for deterministic tests.
func backoffDelay(attempt int, retryAfter string, rnd func() float64) time.Duration {
	if d, ok := parseRetryAfter(retryAfter); ok {
		return d
	}
	d := baseBackoff << attempt
	if d > maxBackoff || d <= 0 {
		d = maxBackoff
	}
	// Full jitter over [d/2, d) spreads a thundering herd of council members.
	jittered := time.Duration(float64(d) * (0.5 + 0.5*rnd()))
	if jittered > maxBackoff {
		jittered = maxBackoff
	}
	return jittered
}

// retryAfterHeader reads Retry-After off a response that may be nil.
func retryAfterHeader(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get("Retry-After")
}
