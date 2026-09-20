package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// decodeJSON reads a size-capped JSON body, rejecting unknown fields so a typo
// in a client fails loudly instead of being ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, into any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errTooLarge
		}
		return badRequest("body must be valid JSON matching the documented shape")
	}
	return nil
}

// pathID reads a numeric {id}; anything else cannot exist, so it is a 404.
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, errNotFound
	}
	return id, nil
}
