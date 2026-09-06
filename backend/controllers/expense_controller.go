package controllers

import (
	"encoding/json"
	"io"
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

func (c *ExpenseController) CreateAutomationEntryV1(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "invalid request body"},
		})
		return
	}

	log.WithField("automationPayload", string(bodyBytes)).Info("automation entry payload")

	req := &request.CreateAutomationEntryRequest{}
	if err := json.Unmarshal(bodyBytes, req); err != nil {
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

	record, err := c.Service.CreateAutomationEntry(r.Context(), authUserID, req)
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

func (c *ExpenseController) UpdateExpenseV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	transactionID := strings.TrimSpace(mux.Vars(r)["transactionId"])
	if transactionID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "transactionId is required"},
		})
		return
	}

	req := &request.UpdateExpenseRequest{}
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

	record, err := c.Service.UpdateExpense(r.Context(), groupID, transactionID, authUserID, req)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.ExpenseResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         record,
	})
}

func (c *ExpenseController) DeleteExpenseV1(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "groupId is required"},
		})
		return
	}

	transactionID := strings.TrimSpace(mux.Vars(r)["transactionId"])
	if transactionID == "" {
		c.writeJSON(w, http.StatusBadRequest, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "transactionId is required"},
		})
		return
	}

	authUserID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	if strings.TrimSpace(authUserID) == "" {
		c.writeJSON(w, http.StatusUnauthorized, response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: "unauthorized"},
		})
		return
	}

	record, err := c.Service.DeleteExpense(r.Context(), groupID, transactionID)
	if err != nil {
		c.writeJSON(w, errorStatus(err), response.ExpenseResponse{
			BaseResponse: response.BaseResponse{Success: false, Error: err.Error()},
		})
		return
	}

	c.writeJSON(w, http.StatusOK, response.ExpenseResponse{
		BaseResponse: response.BaseResponse{Success: true},
		Data:         record,
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

func (c *ExpenseController) CreateStatementIngestionV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	req := &request.CreateStatementIngestionRequest{}
	if err := c.decodeStrictJSON(req, r); err != nil {
		c.writeStatementError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	detail, err := c.Service.CreateStatementIngestion(r.Context(), groupID, userID, req)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusCreated, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) ListStatementIngestionsV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	records, err := c.Service.ListStatementIngestions(r.Context(), groupID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionListResponse{BaseResponse: response.BaseResponse{Success: true}, Data: records})
}

func (c *ExpenseController) GetStatementIngestionV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	ingestionID := strings.TrimSpace(mux.Vars(r)["ingestionId"])
	if ingestionID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId is required")
		return
	}
	detail, err := c.Service.GetStatementIngestion(r.Context(), groupID, ingestionID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) UpdateStatementIngestionRowV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	ingestionID, rowID := strings.TrimSpace(vars["ingestionId"]), strings.TrimSpace(vars["rowId"])
	if ingestionID == "" || rowID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId and rowId are required")
		return
	}
	req := &request.UpdateStatementIngestionRowRequest{}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := c.decodeStrictJSON(req, r); err != nil {
		c.writeStatementError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	detail, err := c.Service.UpdateStatementIngestionRow(r.Context(), groupID, ingestionID, rowID, userID, req)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) DeleteStatementIngestionRowV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	ingestionID, rowID := strings.TrimSpace(vars["ingestionId"]), strings.TrimSpace(vars["rowId"])
	if ingestionID == "" || rowID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId and rowId are required")
		return
	}
	detail, err := c.Service.DeleteStatementIngestionRow(r.Context(), groupID, ingestionID, rowID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) CreateCombinedStatementMatchV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	ingestionID := strings.TrimSpace(mux.Vars(r)["ingestionId"])
	if ingestionID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId is required")
		return
	}
	req := &request.CreateCombinedStatementMatchRequest{}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := c.decodeStrictJSON(req, r); err != nil {
		c.writeStatementError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	detail, err := c.Service.CreateCombinedStatementMatch(r.Context(), groupID, ingestionID, userID, req)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) DissolveCombinedStatementMatchV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	ingestionID, combinedMatchID := strings.TrimSpace(vars["ingestionId"]), strings.TrimSpace(vars["combinedMatchId"])
	if ingestionID == "" || combinedMatchID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId and combinedMatchId are required")
		return
	}
	detail, err := c.Service.DissolveCombinedStatementMatch(r.Context(), groupID, ingestionID, combinedMatchID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) ConfirmStatementIngestionV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	ingestionID := strings.TrimSpace(mux.Vars(r)["ingestionId"])
	if ingestionID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId is required")
		return
	}
	detail, err := c.Service.ConfirmStatementIngestion(r.Context(), groupID, ingestionID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) DeleteStatementIngestionV1(w http.ResponseWriter, r *http.Request) {
	groupID, userID, ok := c.statementRequestScope(w, r)
	if !ok {
		return
	}
	ingestionID := strings.TrimSpace(mux.Vars(r)["ingestionId"])
	if ingestionID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "ingestionId is required")
		return
	}
	detail, err := c.Service.DeleteStatementIngestion(r.Context(), groupID, ingestionID, userID)
	if err != nil {
		c.writeStatementError(w, errorStatus(err), err.Error())
		return
	}
	c.writeJSON(w, http.StatusOK, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: true}, Data: detail})
}

func (c *ExpenseController) statementRequestScope(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	groupID := strings.TrimSpace(mux.Vars(r)["groupId"])
	if groupID == "" {
		c.writeStatementError(w, http.StatusBadRequest, "groupId is required")
		return "", "", false
	}
	userID, _ := r.Context().Value(constants.AuthUserIDCtx).(string)
	userID = strings.TrimSpace(userID)
	if userID == "" {
		c.writeStatementError(w, http.StatusUnauthorized, "unauthorized")
		return "", "", false
	}
	return groupID, userID, true
}

func (c *ExpenseController) writeStatementError(w http.ResponseWriter, status int, message string) {
	c.writeJSON(w, status, response.StatementIngestionResponse{BaseResponse: response.BaseResponse{Success: false, Error: message}})
}

func errorStatus(err error) int {
	message := strings.ToLower(err.Error())

	switch {
	case strings.Contains(message, "not found"):
		return http.StatusNotFound
	case strings.Contains(message, "changed during"),
		strings.Contains(message, "cannot be edited"),
		strings.Contains(message, "not ready for confirmation"),
		strings.Contains(message, "already assigned"):
		return http.StatusConflict
	case strings.Contains(message, "required"),
		strings.Contains(message, "must be"),
		strings.Contains(message, "at most"),
		strings.Contains(message, "duplicated"),
		strings.Contains(message, "type mismatch"),
		strings.Contains(message, "greater than"),
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
