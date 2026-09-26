package handler

import (
	"context"
	"net/http"
	"time"
)

// contextWithTimeout membuat context berbatas waktu dari request.
func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}
