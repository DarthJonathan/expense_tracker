package controllers

import (
	"encoding/json"
	"net/http"
)

type BaseController struct{}

func (c *BaseController) decodeJSON(v any, r *http.Request) error {
	if r.Body == http.NoBody {
		return nil
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func (c *BaseController) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
