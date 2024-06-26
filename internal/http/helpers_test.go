package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
)

// chiRequest builds a request with a chi RouteContext carrying the given
// URL param, so handler tests can call handlers directly without going
// through the full router (and its auth middleware).
func chiRequest(method, target, paramKey, paramValue string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramValue)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}
