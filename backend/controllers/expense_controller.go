package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"expense-tracker/backend/constants"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"
	"expense-tracker/backend/service"

	"github.com/apex/log"
	"github.com/gorilla/mux"
)

type ExpenseController struct {
	BaseController
	Service *service.ExpenseService
}

func NewExpenseController(expenseService *service.ExpenseService) *ExpenseController {
	return &ExpenseController{Service: expenseService}
}

func (c *ExpenseController) CreateGroupV1(w http.ResponseWriter, r *http.Request) {
	req := &request.CreateGroupRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.GroupResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	group, err := c.Service.CreateGroup(r.Context(), req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.GroupResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.GroupResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         group,
	})
}

func (c *ExpenseController) CreateAccountV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.AccountResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	req := &request.CreateAccountRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.AccountResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	account, err := c.Service.CreateAccount(r.Context(), groupID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.AccountResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.AccountResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         account,
	})
}

func (c *ExpenseController) ListAccountsV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.AccountListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	accounts, err := c.Service.ListAccounts(r.Context(), groupID)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.AccountListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.AccountListResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         accounts,
	})
}

func (c *ExpenseController) CreateCategoryV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.CategoryResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	req := &request.CreateCategoryRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.CategoryResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.CategoryResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	category, err := c.Service.CreateCategory(r.Context(), groupID, authUserID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.CategoryResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.CategoryResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         category,
	})
}

func (c *ExpenseController) ListCategoriesV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.CategoryListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.CategoryListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	categories, err := c.Service.ListCategories(r.Context(), groupID, authUserID)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.CategoryListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.CategoryListResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         categories,
	})
}

func (c *ExpenseController) CreateExpenseV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	req := &request.CreateExpenseRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	record, err := c.Service.CreateExpense(r.Context(), groupID, authUserID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.ExpenseResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         record,
	})
}

func (c *ExpenseController) CreateEntryV1(w http.ResponseWriter, r *http.Request) {
	req := &request.CreateEntryRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	group, err := c.Service.ResolveOrCreateUserGroup(r.Context(), authUserID)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	record, err := c.Service.CreateExpense(r.Context(), group.ID, authUserID, &request.CreateExpenseRequest{
		AccountID:  req.AccountID,
		CategoryID: req.CategoryID,
		Type:       req.Type,
		Amount:     req.Amount,
		Currency:   req.Currency,
		OccurredOn: req.OccurredOn,
		Merchant:   req.Merchant,
		Note:       req.Note,
	})
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.ExpenseResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         record,
	})
}

func (c *ExpenseController) ListExpensesV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	req := &request.ListExpensesRequest{
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
		Type:  strings.TrimSpace(r.URL.Query().Get("type")),
	}
	if monthsBackRaw := strings.TrimSpace(r.URL.Query().Get("monthsBack")); monthsBackRaw != "" {
		monthsBack, err := strconv.Atoi(monthsBackRaw)
		if err != nil || monthsBack < 0 {
			c.writeJSON(w, http.StatusBadRequest, response.ExpenseListResponse{
				BaseResponse: response.BaseResponse{Success: false, Error: "monthsBack must be a non-negative integer"},
			})
			return
		}
		req.MonthsBack = monthsBack
	}
	if limitRaw := strings.TrimSpace(r.URL.Query().Get("limit")); limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)
		if err != nil || limit < 1 {
			c.writeJSON(w, http.StatusBadRequest, response.ExpenseListResponse{
				BaseResponse: response.BaseResponse{Success: false, Error: "limit must be a positive integer"},
			})
			return
		}
		req.Limit = limit
	}

	records, err := c.Service.ListExpenses(r.Context(), groupID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.ExpenseListResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         records,
	})
}

func (c *ExpenseController) CreateAdjustmentV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.AdjustmentResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	req := &request.CreateAdjustmentRequest{}
	if err := c.decodeJSON(req, r); err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.AdjustmentResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.AdjustmentResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	record, err := c.Service.CreateAdjustment(r.Context(), groupID, authUserID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.AdjustmentResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusCreated, response.AdjustmentResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         record,
	})
}

func (c *ExpenseController) ListAdjustmentsV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.AdjustmentListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	records, err := c.Service.ListAdjustments(r.Context(), groupID)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.AdjustmentListResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.AdjustmentListResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         records,
	})
}

func errorStatus(err error) int {
	message := strings.ToLower(err.Error())

	switch {
	case strings.Contains(message, "required"),
		strings.Contains(message, "must be"),
		strings.Contains(message, "invalid input syntax for type uuid"),
		strings.Contains(message, "violates foreign key constraint"),
		strings.Contains(message, "violates check constraint"),
		strings.Contains(message, "duplicate key"):
		return http.StatusBadRequest
	default:
		log.WithError(err).Warn("unexpected backend error")
		return http.StatusInternalServerError
	}
}
