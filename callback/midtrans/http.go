package midtrans

import (
	"net/http"

	"github.com/rs/zerolog"
)

func validateHeaders(logger zerolog.Logger, header http.Header) error {
	ct := header.Get("Content-Type")
	if ct == "" {
		return ErrContenTypeIsRequired
	}
	if ct != "application/json" {
		return ErrInvalidContentType
	}
	return nil
}

func writeJSONResponse(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
}
