// Package metrics exposes accounts-api's Prometheus endpoint. It's a thin
// pass-through over cauri-go-kit/metrics so the rest of the service (and
// its tests) depend on our own package instead of reaching into the kit
// directly — if we ever need a service-specific metric this is where it
// goes.
package metrics

import (
	"net/http"

	kitmetrics "github.com/demo-org-np-migration/cauri-go-kit/metrics"
)

// Handler serves the Prometheus text exposition at GET /metrics.
func Handler() http.Handler {
	return kitmetrics.Handler()
}

// Middleware records request duration by route and status.
func Middleware(next http.Handler) http.Handler {
	return kitmetrics.Middleware(next)
}
