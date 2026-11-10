package requestparams

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

func PathID(r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0 && strconv.FormatInt(id, 10) == raw
}
