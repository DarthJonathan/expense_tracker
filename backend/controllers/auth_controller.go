package controllers

import (
	"net/http"
	"strings"

	"expense-tracker/backend/constants"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"
	"expense-tracker/backend/service"
)

type AuthController struct {
	BaseController
	Service *service.AuthService
}

func NewAuthController(authService *service.AuthService) *AuthController {
	return &AuthController{Service: authService}
}

func (c *AuthController) RegisterV1(w http.ResponseWriter, r *http.Request) {
	req := &request.RegisterRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	data, err := c.Service.Register(r.Context(), req)
	if err != nil {
		c.writeJSON(w, authErrorStatus(err), response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.AuthResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         data,
	})
}

func (c *AuthController) LoginV1(w http.ResponseWriter, r *http.Request) {
	req := &request.LoginRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	data, err := c.Service.Login(r.Context(), req)
	if err != nil {
		c.writeJSON(w, authErrorStatus(err), response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.AuthResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         data,
	})
}

func (c *AuthController) AuthenticateAPIKeyV1(w http.ResponseWriter, r *http.Request) {
	req := &request.AuthenticateAPIKeyRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	data, err := c.Service.AuthenticateWithAPIKey(r.Context(), req.APIKey)
	if err != nil {
		c.writeJSON(w, authErrorStatus(err), response.AuthResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.AuthResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         data,
	})
}

func (c *AuthController) CreateAPIKeyV1(w http.ResponseWriter, r *http.Request) {
	req := &request.CreateAPIKeyRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.APIKeyResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	userID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	if strings.TrimSpace(userID) == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.APIKeyResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	data, err := c.Service.CreateAPIKey(r.Context(), userID, req)
	if err != nil {
		c.writeJSON(w, authErrorStatus(err), response.APIKeyResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.APIKeyResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         data,
	})
}

func authErrorStatus(err error) int {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "required"),
		strings.Contains(message, "invalid credentials"),
		strings.Contains(message, "invalid api key"),
		strings.Contains(message, "api key expired"),
		strings.Contains(message, "already registered"),
		strings.Contains(message, "at least"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
