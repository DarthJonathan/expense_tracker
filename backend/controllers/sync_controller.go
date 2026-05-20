package controllers

import (
	"net/http"
	"strings"
	"time"

	"expense-tracker/backend/constants"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"
	"expense-tracker/backend/service"

	"github.com/apex/log"
)

type SyncController struct {
	BaseController
	Service *service.SyncService
}

func NewSyncController(syncService *service.SyncService) *SyncController {
	return &SyncController{Service: syncService}
}

func (c *SyncController) HealthV1(w http.ResponseWriter, _ *http.Request) {
	c.writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

func (c *SyncController) SyncV1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		c.writeJSON(w, http.StatusMethodNotAllowed, response.SyncResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "method not allowed"},
		})
		return
	}

	req := &request.SyncRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.SyncResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.SyncResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	result, err := c.Service.Sync(r.Context(), authUserID, req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "required") {
			status = http.StatusBadRequest
		}

		log.WithError(err).WithFields(log.Fields{
			"route":  "/api/v1/sync",
			"status": status,
		}).Warn("sync failed")

		c.writeJSON(w, status, response.SyncResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.SyncResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         result,
	})
}
