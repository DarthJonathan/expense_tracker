package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type BaseController struct{}

func (c *BaseController) decodeJSON(v any, r *http.Request) error {
	if r.Body == http.NoBody {
		return nil
	}

	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

func (c *BaseController) decodeStrictJSON(v any, r *http.Request) error {
	if r.Body == http.NoBody {
		return nil
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

func (c *BaseController) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
