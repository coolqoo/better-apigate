package v1

import (
	"net/http"
	"strconv"

	"github.com/artpar/apigate/pkg/jsonapi"
)

// History uses the same bounded JSON:API pagination as module collections.
func requestPagination(w http.ResponseWriter, r *http.Request) (*jsonapi.Pagination, bool) {
	page, size := 1, 50
	for _, field := range []struct {
		name string
		out  *int
	}{{"page[number]", &page}, {"page[size]", &size}} {
		if value := r.URL.Query().Get(field.name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				jsonapi.WriteBadRequest(w, "Pagination values must be positive integers.")
				return nil, false
			}
			*field.out = n
		}
	}
	if size > 100 || page > int(^uint(0)>>1)/size {
		jsonapi.WriteBadRequest(w, "Use a page size of 1–100 and a valid page number.")
		return nil, false
	}
	return jsonapi.NewPagination(0, page, size, r.URL.RequestURI()), true
}
