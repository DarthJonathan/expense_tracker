package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	statementStatusParsed         = "parsed"
	statementStatusMatchingReview = "matching_review"
	statementStatusReady          = "ready"
	statementStatusConfirmed      = "confirmed"
	statementStatusDeleted        = "deleted"
)

func (s *ExpenseService) CreateStatementIngestion(ctx context.Context, groupID, userID string, req *request.CreateStatementIngestionRequest) (*response.StatementIngestionDetail, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	if len(clientRequestID) > 128 {
		return nil, fmt.Errorf("clientRequestId must be at most 128 characters")
	}
	sourceFingerprint := strings.ToLower(strings.TrimSpace(req.SourceFingerprint))
	if sourceFingerprint != "" {
		decoded, decodeErr := hex.DecodeString(sourceFingerprint)
		if decodeErr != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("sourceFingerprint must be a SHA-256 hex digest")
		}
	}
	accountID := strings.TrimSpace(req.AccountID)
	if err := s.ensureActiveStatementAccount(ctx, groupID, accountID); err != nil {
		return nil, err
	}
	if len(req.Rows) > 5000 {
		return nil, fmt.Errorf("at most 5000 statement rows are allowed")
	}
	sourceName := strings.TrimSpace(req.SourceName)
	if sourceName == "" {
		return nil, fmt.Errorf("sourceName is required")
	}
	if len(sourceName) > 255 {
		return nil, fmt.Errorf("sourceName must be at most 255 characters")
	}
	institution := strings.TrimSpace(req.Institution)
	if len(institution) > 120 {
		return nil, fmt.Errorf("institution must be at most 120 characters")
	}
	statementCurrency, err := normalizeCurrencyCode(req.StatementCurrency)
	if err != nil {
		return nil, err
	}
	periodStart, err := optionalStatementDate(req.PeriodStart)
	if err != nil {
		return nil, fmt.Errorf("periodStart: %w", err)
	}
	periodEnd, err := optionalStatementDate(req.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("periodEnd: %w", err)
	}
	if periodStart != nil && periodEnd != nil && *periodStart > *periodEnd {
		return nil, fmt.Errorf("periodStart must not be after periodEnd")
	}
	if existingID, err := s.findExistingStatementIngestion(ctx, groupID, clientRequestID, sourceFingerprint); err != nil {
		return nil, err
	} else if existingID != "" {
		return s.getStatementIngestion(ctx, groupID, existingID, false)
	}
	statementDate, err := optionalStatementDate(req.StatementDate)
	if err != nil {
		return nil, fmt.Errorf("statementDate: %w", err)
	}
	paymentDueDate, err := optionalStatementDate(req.PaymentDueDate)
	if err != nil {
		return nil, fmt.Errorf("paymentDueDate: %w", err)
	}

	ingestionID, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate statement ingestion id: %w", err)
	}
	now := time.Now().UTC()
	rows := make([]dao.ExpenseStatementIngestionRow, 0, len(req.Rows))
	seenKeys := make(map[string]struct{}, len(req.Rows))
	validatedAccounts := map[string]struct{}{accountID: {}}
	validatedCategories := make(map[string]struct{})
	for i, input := range req.Rows {
		row, err := normalizeStatementIngestionRow(input, ingestionID, groupID, accountID, statementCurrency, now)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		if _, duplicate := seenKeys[row.SourceRowKey]; duplicate {
			return nil, fmt.Errorf("row %d: sourceRowKey is duplicated", i+1)
		}
		rowAccountID := derefString(row.AccountID)
		if _, validated := validatedAccounts[rowAccountID]; !validated {
			if err := s.ensureActiveStatementAccount(ctx, groupID, rowAccountID); err != nil {
				return nil, fmt.Errorf("row %d: %w", i+1, err)
			}
			validatedAccounts[rowAccountID] = struct{}{}
		}
		if categoryID := derefString(row.CategoryID); categoryID != "" {
			categoryKey := row.Type + "|" + categoryID
			if _, validated := validatedCategories[categoryKey]; !validated {
				if err := s.ensureActiveStatementCategory(ctx, groupID, categoryID, userID, row.Type); err != nil {
					return nil, fmt.Errorf("row %d: %w", i+1, err)
				}
				validatedCategories[categoryKey] = struct{}{}
			}
		}
		seenKeys[row.SourceRowKey] = struct{}{}
		rows = append(rows, row)
	}
	if err := s.suggestStatementMatches(ctx, groupID, rows); err != nil {
		return nil, fmt.Errorf("suggest statement matches: %w", err)
	}
	if err := s.suggestStatementCategories(ctx, groupID, userID, rows); err != nil {
		return nil, fmt.Errorf("suggest statement categories: %w", err)
	}

	cardholderControls := normalizeCardholderControls(req.Cardholders)
	validation := buildStatementValidation(rows, statementCurrency, req.PreviousBalance, req.DeclaredNewTransactionsTotal, req.StatementGrandTotal, cardholderControls)
	status := statementStatusParsed
	if len(rows) > 0 {
		status = statementStatusMatchingReview
	}
	createdBy := strings.TrimSpace(userID)
	ingestion := &dao.ExpenseStatementIngestion{
		ID:                           ingestionID,
		GroupID:                      groupID,
		AccountID:                    accountID,
		ClientRequestID:              clientRequestID,
		SourceFingerprint:            sourceFingerprint,
		Status:                       status,
		SourceName:                   sourceName,
		Institution:                  institution,
		StatementCurrency:            statementCurrency,
		StatementDate:                statementDate,
		PaymentDueDate:               paymentDueDate,
		PeriodStart:                  periodStart,
		PeriodEnd:                    periodEnd,
		PreviousBalance:              req.PreviousBalance,
		DeclaredNewTransactionsTotal: req.DeclaredNewTransactionsTotal,
		StatementGrandTotal:          req.StatementGrandTotal,
		ParsedRowCount:               len(rows),
		CardholderControls:           cardholderControls,
		Validation:                   validation,
		Warnings:                     normalizeStatementWarnings(req.Warnings),
		CreatedBy:                    &createdBy,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table((dao.ExpenseStatementIngestion{}).TableName()).Create(ingestion).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			return tx.Table((dao.ExpenseStatementIngestionRow{}).TableName()).Create(&rows).Error
		}
		return nil
	}); err != nil {
		if existingID, lookupErr := s.findExistingStatementIngestion(ctx, groupID, clientRequestID, sourceFingerprint); lookupErr == nil && existingID != "" {
			return s.getStatementIngestion(ctx, groupID, existingID, false)
		}
		return nil, err
	}
	return &response.StatementIngestionDetail{Ingestion: *ingestion, Rows: rows}, nil
}

func (s *ExpenseService) ListStatementIngestions(ctx context.Context, groupID, userID string) ([]dao.ExpenseStatementIngestion, error) {
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	records := make([]dao.ExpenseStatementIngestion, 0)
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, group_id::text as group_id, account_id::text as account_id, client_request_id, source_fingerprint, status, source_name, institution,
			statement_currency, to_char(statement_date, 'YYYY-MM-DD') as statement_date,
			to_char(payment_due_date, 'YYYY-MM-DD') as payment_due_date, to_char(period_start, 'YYYY-MM-DD') as period_start,
			to_char(period_end, 'YYYY-MM-DD') as period_end, previous_balance, declared_new_transactions_total,
			statement_grand_total, parsed_row_count, cardholder_controls, validation, warnings, created_by::text as created_by,
			confirmed_at, created_at, updated_at, deleted_at
		from %s where group_id = ?::uuid and deleted_at is null
		order by updated_at desc
	`, dao.QualifiedTable("expense_statement_ingestions")), groupID).Scan(&records).Error
	return records, err
}

func (s *ExpenseService) findExistingStatementIngestion(ctx context.Context, groupID, clientRequestID, sourceFingerprint string) (string, error) {
	if clientRequestID == "" && sourceFingerprint == "" {
		return "", nil
	}
	query := s.DB.WithContext(ctx).Table((dao.ExpenseStatementIngestion{}).TableName()).
		Select("id::text").
		Where("group_id = ?::uuid and deleted_at is null", groupID)
	switch {
	case clientRequestID != "" && sourceFingerprint != "":
		query = query.Where("client_request_id = ? or source_fingerprint = ?", clientRequestID, sourceFingerprint)
	case clientRequestID != "":
		query = query.Where("client_request_id = ?", clientRequestID)
	default:
		query = query.Where("source_fingerprint = ?", sourceFingerprint)
	}
	var id string
	if err := query.Order("created_at desc").Limit(1).Scan(&id).Error; err != nil {
		return "", err
	}
	return strings.TrimSpace(id), nil
}

func (s *ExpenseService) GetStatementIngestion(ctx context.Context, groupID, ingestionID, userID string) (*response.StatementIngestionDetail, error) {
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	return s.getStatementIngestion(ctx, groupID, ingestionID, false)
}

func (s *ExpenseService) UpdateStatementIngestionRow(ctx context.Context, groupID, ingestionID, rowID, userID string, req *request.UpdateStatementIngestionRowRequest) (*response.StatementIngestionDetail, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	detail, err := s.getStatementIngestion(ctx, groupID, ingestionID, false)
	if err != nil {
		return nil, err
	}
	if detail.Ingestion.Status == statementStatusConfirmed {
		return nil, fmt.Errorf("confirmed statement ingestion cannot be edited")
	}
	row := findStatementIngestionRow(detail.Rows, rowID)
	if row == nil {
		return nil, fmt.Errorf("statement row not found")
	}
	next := *row
	if req.OccurredOn != nil {
		value, err := optionalStatementDate(*req.OccurredOn)
		if err != nil {
			return nil, fmt.Errorf("occurredOn: %w", err)
		}
		next.OccurredOn = value
	}
	if req.Merchant != nil {
		next.Merchant = strings.TrimSpace(*req.Merchant)
	}
	if req.Amount != nil {
		if *req.Amount < 0 {
			return nil, fmt.Errorf("amount must be >= 0")
		}
		next.Amount = *req.Amount
	}
	if req.Currency != nil {
		currency, err := normalizeCurrencyCode(*req.Currency)
		if err != nil {
			return nil, err
		}
		next.Currency = currency
	}
	if req.ForeignAmount != nil {
		if *req.ForeignAmount < 0 {
			return nil, fmt.Errorf("foreignAmount must be >= 0")
		}
		next.ForeignAmount = req.ForeignAmount
	}
	if req.ForeignCurrency != nil {
		foreignCurrency := strings.TrimSpace(*req.ForeignCurrency)
		if foreignCurrency == "" {
			next.ForeignCurrency = ""
			next.ForeignAmount = nil
		} else {
			currency, err := normalizeCurrencyCode(foreignCurrency)
			if err != nil {
				return nil, fmt.Errorf("foreignCurrency: %w", err)
			}
			next.ForeignCurrency = currency
		}
	}
	if next.ForeignAmount != nil && next.ForeignCurrency == "" {
		return nil, fmt.Errorf("foreignCurrency is required with foreignAmount")
	}
	if req.StatementKind != nil {
		kind, err := normalizeStatementKind(*req.StatementKind)
		if err != nil {
			return nil, err
		}
		next.StatementKind = kind
	}
	if req.Type != nil {
		nextType := strings.ToLower(strings.TrimSpace(*req.Type))
		if nextType != "expense" && nextType != "income" {
			return nil, fmt.Errorf("type must be expense or income")
		}
		next.Type = nextType
	}
	if req.AccountID != nil {
		accountID := strings.TrimSpace(*req.AccountID)
		if err := s.ensureActiveStatementAccount(ctx, groupID, accountID); err != nil {
			return nil, err
		}
		next.AccountID = stringPointer(accountID)
	}
	if req.CategoryID != nil {
		categoryID := strings.TrimSpace(*req.CategoryID)
		if categoryID == "" {
			next.CategoryID = nil
		} else if err := s.ensureActiveStatementCategory(ctx, groupID, categoryID, userID, next.Type); err != nil {
			return nil, err
		} else {
			next.CategoryID = stringPointer(categoryID)
		}
	}
	if req.Note != nil {
		next.Note = strings.TrimSpace(*req.Note)
	}
	if req.WarningCodes != nil {
		next.WarningCodes = normalizeStatementWarnings(*req.WarningCodes)
	}
	if req.ReviewStatus != nil {
		status := strings.ToLower(strings.TrimSpace(*req.ReviewStatus))
		if status != "unreviewed" && status != "new" && status != "matched" && status != "ignored" {
			return nil, fmt.Errorf("reviewStatus must be unreviewed, new, matched, or ignored")
		}
		next.ReviewStatus = status
	}
	if req.MatchExpenseID != nil {
		matchID := strings.TrimSpace(*req.MatchExpenseID)
		if matchID == "" {
			next.MatchExpenseID = nil
		} else if err := s.ensureMatchExpense(ctx, groupID, matchID); err != nil {
			return nil, err
		} else {
			next.MatchExpenseID = stringPointer(matchID)
		}
	}
	if next.ReviewStatus == "matched" && (next.MatchExpenseID == nil || strings.TrimSpace(*next.MatchExpenseID) == "") {
		return nil, fmt.Errorf("matchExpenseId is required when reviewStatus is matched")
	}
	if next.ReviewStatus != "matched" {
		next.MatchExpenseID = nil
	}
	if next.ReviewStatus == "matched" {
		if err := s.ensureNoDuplicateStatementMatch(ctx, groupID, ingestionID, rowID, derefString(next.MatchExpenseID)); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	next.ReviewedBy = stringPointer(strings.TrimSpace(userID))
	next.ReviewedAt = &now
	next.UpdatedAt = now
	nextRows := replaceStatementIngestionRow(detail.Rows, next)
	validation := buildStatementValidation(nextRows, detail.Ingestion.StatementCurrency, detail.Ingestion.PreviousBalance, detail.Ingestion.DeclaredNewTransactionsTotal, detail.Ingestion.StatementGrandTotal, detail.Ingestion.CardholderControls)
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedStatus string
		if err := tx.Raw(fmt.Sprintf(`select status from %s where id = ?::uuid and group_id = ?::uuid and deleted_at is null for update`, dao.QualifiedTable("expense_statement_ingestions")), ingestionID, groupID).Scan(&lockedStatus).Error; err != nil {
			return err
		}
		if lockedStatus == "" {
			return fmt.Errorf("statement ingestion not found")
		}
		if lockedStatus == statementStatusConfirmed {
			return fmt.Errorf("confirmed statement ingestion cannot be edited")
		}
		result := tx.Table((dao.ExpenseStatementIngestionRow{}).TableName()).
			Where("id = ?::uuid and ingestion_id = ?::uuid and group_id = ?::uuid and updated_at = ? and deleted_at is null", rowID, ingestionID, groupID, row.UpdatedAt).
			Updates(map[string]any{
				"occurred_on": next.OccurredOn, "merchant": next.Merchant, "amount": next.Amount,
				"currency": next.Currency, "foreign_amount": next.ForeignAmount, "foreign_currency": next.ForeignCurrency,
				"statement_kind": next.StatementKind, "type": next.Type, "account_id": next.AccountID, "category_id": next.CategoryID,
				"note": next.Note, "warning_codes": next.WarningCodes, "review_status": next.ReviewStatus,
				"match_expense_id": next.MatchExpenseID, "reviewed_by": next.ReviewedBy, "reviewed_at": next.ReviewedAt,
				"updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("statement row changed during review; reload and retry")
		}
		status := statementIngestionStatus(detail.Rows, rowID, next)
		ingestionResult := tx.Table((dao.ExpenseStatementIngestion{}).TableName()).
			Where("id = ?::uuid and group_id = ?::uuid and updated_at = ? and deleted_at is null", ingestionID, groupID, detail.Ingestion.UpdatedAt).
			Updates(map[string]any{"status": status, "validation": validation, "updated_at": now})
		if ingestionResult.Error != nil {
			return ingestionResult.Error
		}
		if ingestionResult.RowsAffected != 1 {
			return fmt.Errorf("statement ingestion changed during review; reload and retry")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.getStatementIngestion(ctx, groupID, ingestionID, false)
}

func (s *ExpenseService) ConfirmStatementIngestion(ctx context.Context, groupID, ingestionID, userID string) (*response.StatementIngestionDetail, error) {
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	detail, err := s.getStatementIngestion(ctx, groupID, ingestionID, false)
	if err != nil {
		return nil, err
	}
	if detail.Ingestion.Status == statementStatusConfirmed {
		return detail, nil
	}
	if detail.Ingestion.Status != statementStatusReady {
		return nil, fmt.Errorf("statement ingestion is not ready for confirmation")
	}
	baseCurrency, err := s.resolveUserBaseCurrency(ctx, userID)
	if err != nil {
		return nil, err
	}
	rates := make(map[string]statementConfirmationRate, len(detail.Rows))
	for _, row := range detail.Rows {
		if err := s.validateStatementRowForConfirmation(ctx, groupID, userID, row); err != nil {
			return nil, fmt.Errorf("row %s: %w", row.SourceRowKey, err)
		}
		if row.ReviewStatus == "ignored" {
			continue
		}
		// FX resolution may use a remote provider. Resolve it before the database
		// transaction so locks are held only for local database work.
		baseAmount, fxRate, fxRateDate, err := s.convertToBaseAmount(ctx, row.Amount, row.Currency, baseCurrency, derefStatementDate(row.OccurredOn))
		if err != nil {
			return nil, err
		}
		rates[row.ID] = statementConfirmationRate{RowUpdatedAt: row.UpdatedAt, BaseAmount: baseAmount, FxRate: fxRate, FxRateDate: fxRateDate}
	}

	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lockedDetail, err := loadStatementIngestionForUpdate(ctx, tx, groupID, ingestionID)
		if err != nil {
			return err
		}
		if lockedDetail.Ingestion.Status == statementStatusConfirmed {
			return nil
		}
		if lockedDetail.Ingestion.Status != statementStatusReady {
			return fmt.Errorf("statement ingestion is not ready for confirmation")
		}
		matchedExpenseIDs := make(map[string]struct{}, len(lockedDetail.Rows))
		for _, row := range lockedDetail.Rows {
			if err := s.validateStatementRowForConfirmation(ctx, groupID, userID, row); err != nil {
				return fmt.Errorf("row %s: %w", row.SourceRowKey, err)
			}
			if row.ReviewStatus == "ignored" {
				continue
			}
			if row.ReviewStatus == "matched" {
				matchID := derefString(row.MatchExpenseID)
				if _, duplicate := matchedExpenseIDs[matchID]; duplicate {
					return fmt.Errorf("matched transaction is assigned to multiple rows in this ingestion")
				}
				matchedExpenseIDs[matchID] = struct{}{}
			}
			rate, ok := rates[row.ID]
			if !ok || !rate.RowUpdatedAt.Equal(row.UpdatedAt) {
				return fmt.Errorf("statement changed during confirmation; review and retry")
			}
			metadata := map[string]any{
				"statementIngestion": map[string]any{
					"ingestionId": lockedDetail.Ingestion.ID, "rowId": row.ID, "sourceRowKey": row.SourceRowKey,
					"sourceName": lockedDetail.Ingestion.SourceName, "institution": lockedDetail.Ingestion.Institution,
					"sourceFingerprint": lockedDetail.Ingestion.SourceFingerprint,
					"statementDate":     lockedDetail.Ingestion.StatementDate, "statementReference": row.StatementReference,
					"statementKind": row.StatementKind, "cardholder": row.Cardholder,
					"foreignAmount": row.ForeignAmount, "foreignCurrency": row.ForeignCurrency,
				},
			}
			if row.ReviewStatus == "new" {
				entryID, err := uuid.GenerateUUID()
				if err != nil {
					return fmt.Errorf("generate expense id: %w", err)
				}
				entry := dao.ExpenseEntry{ID: entryID, GroupID: groupID, AccountID: derefString(row.AccountID), CategoryID: derefString(row.CategoryID), Type: row.Type, Amount: row.Amount, Currency: row.Currency, BaseAmount: rate.BaseAmount, BaseCurrency: baseCurrency, FxRate: rate.FxRate, FxRateDate: rate.FxRateDate, OccurredOn: derefStatementDate(row.OccurredOn), Merchant: row.Merchant, Note: row.Note, Metadata: metadata, CreatedBy: stringPointer(userID), CreatedAt: now, UpdatedAt: now}
				if err := tx.Table((dao.ExpenseEntry{}).TableName()).Create(&entry).Error; err != nil {
					return err
				}
				if err := upsertMerchant(tx, merchantFromEntry(groupID, entry, now)); err != nil {
					return err
				}
				if err := tx.Table((dao.ExpenseStatementIngestionRow{}).TableName()).Where("id = ?::uuid", row.ID).Updates(map[string]any{"confirmed_expense_id": entryID, "updated_at": now}).Error; err != nil {
					return err
				}
				continue
			}

			current := dao.ExpenseEntry{}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table((dao.ExpenseEntry{}).TableName()).Where("id = ?::uuid and group_id = ?::uuid and deleted_at is null", derefString(row.MatchExpenseID), groupID).First(&current).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("matched transaction not found")
				}
				return err
			}
			current.Metadata = normalizeMetadata(current.Metadata)
			current.Metadata["statementIngestion"] = metadata["statementIngestion"]
			result := tx.Table((dao.ExpenseEntry{}).TableName()).Where("id = ?::uuid and group_id = ?::uuid and deleted_at is null", current.ID, groupID).Updates(map[string]any{
				"account_id": row.AccountID, "category_id": row.CategoryID, "type": row.Type, "amount": row.Amount,
				"currency": row.Currency, "base_amount": rate.BaseAmount, "base_currency": baseCurrency,
				"fx_rate": rate.FxRate, "fx_rate_date": rate.FxRateDate, "occurred_on": row.OccurredOn, "merchant": row.Merchant,
				"note": row.Note, "metadata": current.Metadata, "updated_at": now,
			})
			if result.Error != nil || result.RowsAffected != 1 {
				if result.Error != nil {
					return result.Error
				}
				return fmt.Errorf("matched transaction not found")
			}
			current.AccountID, current.CategoryID, current.Type, current.Amount, current.Currency = derefString(row.AccountID), derefString(row.CategoryID), row.Type, row.Amount, row.Currency
			current.OccurredOn, current.Merchant, current.Note = derefStatementDate(row.OccurredOn), row.Merchant, row.Note
			if err := upsertMerchant(tx, merchantFromEntry(groupID, current, now)); err != nil {
				return err
			}
			if err := tx.Table((dao.ExpenseStatementIngestionRow{}).TableName()).Where("id = ?::uuid", row.ID).Updates(map[string]any{"confirmed_expense_id": current.ID, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		result := tx.Table((dao.ExpenseStatementIngestion{}).TableName()).Where("id = ?::uuid and group_id = ?::uuid and status = ? and deleted_at is null", ingestionID, groupID, statementStatusReady).Updates(map[string]any{"status": statementStatusConfirmed, "confirmed_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("statement ingestion changed during confirmation")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.getStatementIngestion(ctx, groupID, ingestionID, false)
}

type statementConfirmationRate struct {
	RowUpdatedAt time.Time
	BaseAmount   int
	FxRate       float64
	FxRateDate   string
}

func (s *ExpenseService) DeleteStatementIngestion(ctx context.Context, groupID, ingestionID, userID string) (*response.StatementIngestionDetail, error) {
	if err := s.ensureStatementGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	detail, err := s.getStatementIngestion(ctx, groupID, ingestionID, false)
	if err != nil {
		return nil, err
	}
	// Confirmed master expenses are intentionally retained. Only the staging
	// audit rows are soft-deleted with the ingestion.
	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedStatus string
		if err := tx.Raw(fmt.Sprintf(`select status from %s where id = ?::uuid and group_id = ?::uuid and deleted_at is null for update`, dao.QualifiedTable("expense_statement_ingestions")), ingestionID, groupID).Scan(&lockedStatus).Error; err != nil {
			return err
		}
		if lockedStatus == "" {
			return fmt.Errorf("statement ingestion not found")
		}
		if err := tx.Table((dao.ExpenseStatementIngestionRow{}).TableName()).Where("ingestion_id = ?::uuid and group_id = ?::uuid and deleted_at is null", ingestionID, groupID).Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		result := tx.Table((dao.ExpenseStatementIngestion{}).TableName()).Where("id = ?::uuid and group_id = ?::uuid and deleted_at is null", ingestionID, groupID).Updates(map[string]any{"status": statementStatusDeleted, "deleted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("statement ingestion changed during deletion")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	detail.Ingestion.Status = statementStatusDeleted
	detail.Ingestion.DeletedAt = &now
	for i := range detail.Rows {
		detail.Rows[i].DeletedAt = &now
	}
	return detail, nil
}

func (s *ExpenseService) getStatementIngestion(ctx context.Context, groupID, ingestionID string, includeDeleted bool) (*response.StatementIngestionDetail, error) {
	ingestion := dao.ExpenseStatementIngestion{}
	deletedClause := "and deleted_at is null"
	if includeDeleted {
		deletedClause = ""
	}
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, group_id::text as group_id, account_id::text as account_id, client_request_id, source_fingerprint, status, source_name, institution,
			statement_currency, to_char(statement_date, 'YYYY-MM-DD') as statement_date,
			to_char(payment_due_date, 'YYYY-MM-DD') as payment_due_date, to_char(period_start, 'YYYY-MM-DD') as period_start, to_char(period_end, 'YYYY-MM-DD') as period_end,
			previous_balance, declared_new_transactions_total, statement_grand_total, parsed_row_count, cardholder_controls, validation, warnings,
			created_by::text as created_by, confirmed_at, created_at, updated_at, deleted_at
		from %s where id = ?::uuid and group_id = ?::uuid %s limit 1
	`, dao.QualifiedTable("expense_statement_ingestions"), deletedClause), ingestionID, groupID).Scan(&ingestion).Error
	if err != nil {
		return nil, err
	}
	if ingestion.ID == "" {
		return nil, fmt.Errorf("statement ingestion not found")
	}
	rows := make([]dao.ExpenseStatementIngestionRow, 0)
	err = s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, ingestion_id::text as ingestion_id, group_id::text as group_id, source_row_key,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on, merchant, amount, currency, foreign_amount, foreign_currency, statement_kind, type, cardholder, statement_reference,
			account_id::text as account_id, category_id::text as category_id, note, review_status,
			suggested_expense_id::text as suggested_expense_id, match_confidence,
			match_expense_id::text as match_expense_id, confirmed_expense_id::text as confirmed_expense_id, warning_codes,
			reviewed_by::text as reviewed_by, reviewed_at, created_at, updated_at, deleted_at
		from %s where ingestion_id = ?::uuid and group_id = ?::uuid %s order by occurred_on nulls last, source_row_key
	`, dao.QualifiedTable("expense_statement_ingestion_rows"), deletedClause), ingestionID, groupID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return &response.StatementIngestionDetail{Ingestion: ingestion, Rows: rows}, nil
}

func loadStatementIngestionForUpdate(ctx context.Context, tx *gorm.DB, groupID, ingestionID string) (*response.StatementIngestionDetail, error) {
	ingestion := dao.ExpenseStatementIngestion{}
	err := tx.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, group_id::text as group_id, account_id::text as account_id, client_request_id, source_fingerprint, status, source_name, institution,
			statement_currency, to_char(statement_date, 'YYYY-MM-DD') as statement_date,
			to_char(payment_due_date, 'YYYY-MM-DD') as payment_due_date, to_char(period_start, 'YYYY-MM-DD') as period_start, to_char(period_end, 'YYYY-MM-DD') as period_end,
			previous_balance, declared_new_transactions_total, statement_grand_total, parsed_row_count, cardholder_controls, validation, warnings,
			created_by::text as created_by, confirmed_at, created_at, updated_at, deleted_at
		from %s where id = ?::uuid and group_id = ?::uuid and deleted_at is null for update
	`, dao.QualifiedTable("expense_statement_ingestions")), ingestionID, groupID).Scan(&ingestion).Error
	if err != nil {
		return nil, err
	}
	if ingestion.ID == "" {
		return nil, fmt.Errorf("statement ingestion not found")
	}
	rows := make([]dao.ExpenseStatementIngestionRow, 0)
	err = tx.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, ingestion_id::text as ingestion_id, group_id::text as group_id, source_row_key,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on, merchant, amount, currency, foreign_amount, foreign_currency, statement_kind, type, cardholder, statement_reference,
			account_id::text as account_id, category_id::text as category_id, note, review_status,
			suggested_expense_id::text as suggested_expense_id, match_confidence,
			match_expense_id::text as match_expense_id, confirmed_expense_id::text as confirmed_expense_id, warning_codes,
			reviewed_by::text as reviewed_by, reviewed_at, created_at, updated_at, deleted_at
		from %s where ingestion_id = ?::uuid and group_id = ?::uuid and deleted_at is null order by occurred_on nulls last, source_row_key for update
	`, dao.QualifiedTable("expense_statement_ingestion_rows")), ingestionID, groupID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return &response.StatementIngestionDetail{Ingestion: ingestion, Rows: rows}, nil
}

func normalizeStatementIngestionRow(input request.CreateStatementIngestionRowRequest, ingestionID, groupID, defaultAccountID, defaultCurrency string, now time.Time) (dao.ExpenseStatementIngestionRow, error) {
	key := strings.TrimSpace(input.SourceRowKey)
	if key == "" {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("sourceRowKey is required")
	}
	if len(key) > 160 {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("sourceRowKey must be at most 160 characters")
	}
	occurredOn, err := optionalStatementDate(input.OccurredOn)
	if err != nil {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("occurredOn: %w", err)
	}
	currency := defaultCurrency
	if strings.TrimSpace(input.Currency) != "" {
		currency, err = normalizeCurrencyCode(input.Currency)
		if err != nil {
			return dao.ExpenseStatementIngestionRow{}, err
		}
	}
	foreignCurrency := ""
	if strings.TrimSpace(input.ForeignCurrency) != "" {
		foreignCurrency, err = normalizeCurrencyCode(input.ForeignCurrency)
		if err != nil {
			return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("foreignCurrency: %w", err)
		}
	}
	if input.ForeignAmount != nil && *input.ForeignAmount < 0 {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("foreignAmount must be >= 0")
	}
	if input.ForeignAmount != nil && foreignCurrency == "" {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("foreignCurrency is required with foreignAmount")
	}
	statementKind, err := normalizeStatementKind(input.StatementKind)
	if err != nil {
		return dao.ExpenseStatementIngestionRow{}, err
	}
	entryType := strings.ToLower(strings.TrimSpace(input.Type))
	if entryType == "" {
		entryType = "expense"
	}
	if entryType != "expense" && entryType != "income" {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("type must be expense or income")
	}
	if input.Amount < 0 {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("amount must be >= 0")
	}
	rowID, err := uuid.GenerateUUID()
	if err != nil {
		return dao.ExpenseStatementIngestionRow{}, fmt.Errorf("generate statement row id: %w", err)
	}
	accountID := strings.TrimSpace(input.AccountID)
	if accountID == "" {
		accountID = defaultAccountID
	}
	categoryID := strings.TrimSpace(input.CategoryID)
	return dao.ExpenseStatementIngestionRow{ID: rowID, IngestionID: ingestionID, GroupID: groupID, SourceRowKey: key, OccurredOn: occurredOn, Merchant: strings.TrimSpace(input.Merchant), Amount: input.Amount, Currency: currency, ForeignAmount: input.ForeignAmount, ForeignCurrency: foreignCurrency, StatementKind: statementKind, Type: entryType, Cardholder: strings.TrimSpace(input.Cardholder), StatementReference: strings.TrimSpace(input.StatementReference), AccountID: stringPointer(accountID), CategoryID: nilIfEmpty(categoryID), Note: strings.TrimSpace(input.Note), ReviewStatus: "unreviewed", WarningCodes: normalizeStatementWarnings(input.WarningCodes), CreatedAt: now, UpdatedAt: now}, nil
}

func normalizeStatementKind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	if kind == "" {
		kind = "transaction"
	}
	switch kind {
	case "transaction", "payment", "fee", "refund", "other":
		return kind, nil
	default:
		return "", fmt.Errorf("statementKind must be transaction, payment, fee, refund, or other")
	}
}

type statementMatchCandidate struct {
	ID          string
	OccurredOn  string
	Merchant    string
	MerchantKey string
	Amount      int
	Currency    string
	Type        string
}

type scoredStatementMatch struct {
	RowIndex    int
	CandidateID string
	Score       float64
}

func (s *ExpenseService) suggestStatementMatches(ctx context.Context, groupID string, rows []dao.ExpenseStatementIngestionRow) error {
	if len(rows) == 0 {
		return nil
	}
	var earliest, latest time.Time
	for _, row := range rows {
		if row.OccurredOn == nil {
			continue
		}
		date, err := time.Parse("2006-01-02", *row.OccurredOn)
		if err != nil {
			continue
		}
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
		if latest.IsZero() || date.After(latest) {
			latest = date
		}
	}
	if earliest.IsZero() || latest.IsZero() {
		return nil
	}
	candidates := make([]statementMatchCandidate, 0)
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text as id, to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
			merchant, amount, currency, type
		from %s
		where group_id = ?::uuid and occurred_on between ?::date and ?::date and deleted_at is null
	`, dao.QualifiedTable("expense_entries")), groupID, earliest.AddDate(0, 0, -3).Format("2006-01-02"), latest.AddDate(0, 0, 3).Format("2006-01-02")).Scan(&candidates).Error
	if err != nil {
		return err
	}

	candidateBuckets := make(map[string][]statementMatchCandidate)
	for _, candidate := range candidates {
		candidate.MerchantKey = normalizeMerchantKey(candidate.Merchant)
		key := statementMatchBucket(candidate.Amount, candidate.Currency, candidate.Type)
		candidateBuckets[key] = append(candidateBuckets[key], candidate)
	}
	scored := make([]scoredStatementMatch, 0)
	for rowIndex, row := range rows {
		if row.OccurredOn == nil {
			continue
		}
		rowMerchantKey := normalizeMerchantKey(row.Merchant)
		for _, candidate := range candidateBuckets[statementMatchBucket(row.Amount, row.Currency, row.Type)] {
			score := scoreStatementMatchWithMerchantKeys(row, candidate, rowMerchantKey, candidate.MerchantKey)
			if score >= 0.70 {
				scored = append(scored, scoredStatementMatch{RowIndex: rowIndex, CandidateID: candidate.ID, Score: score})
			}
		}
	}
	sort.SliceStable(scored, func(left, right int) bool {
		if scored[left].Score == scored[right].Score {
			if scored[left].RowIndex == scored[right].RowIndex {
				return scored[left].CandidateID < scored[right].CandidateID
			}
			return scored[left].RowIndex < scored[right].RowIndex
		}
		return scored[left].Score > scored[right].Score
	})
	usedRows := make(map[int]struct{})
	usedCandidates := make(map[string]struct{})
	for _, match := range scored {
		if _, used := usedRows[match.RowIndex]; used {
			continue
		}
		if _, used := usedCandidates[match.CandidateID]; used {
			continue
		}
		candidateID := match.CandidateID
		confidence := math.Round(match.Score*1000) / 1000
		rows[match.RowIndex].SuggestedExpenseID = &candidateID
		rows[match.RowIndex].MatchConfidence = &confidence
		usedRows[match.RowIndex] = struct{}{}
		usedCandidates[match.CandidateID] = struct{}{}
	}
	return nil
}

func (s *ExpenseService) suggestStatementCategories(ctx context.Context, groupID, userID string, rows []dao.ExpenseStatementIngestionRow) error {
	cache := make(map[string]*merchantCategorySuggestion)
	resolved := make(map[string]struct{})
	for index := range rows {
		row := &rows[index]
		if row.CategoryID != nil || strings.TrimSpace(row.Merchant) == "" {
			continue
		}
		cacheKey := strings.Join([]string{row.Type, derefString(row.AccountID), normalizeMerchantKey(row.Merchant), strings.TrimSpace(row.Note)}, "|")
		suggestion := cache[cacheKey]
		if _, found := resolved[cacheKey]; !found {
			var err error
			suggestion, err = s.suggestCategoryForEntry(ctx, groupID, userID, row.Type, derefString(row.AccountID), row.Merchant, row.Note)
			if err != nil {
				return err
			}
			cache[cacheKey] = suggestion
			resolved[cacheKey] = struct{}{}
		}
		if suggestion == nil || strings.TrimSpace(suggestion.CategoryID) == "" {
			continue
		}
		if err := s.ensureActiveStatementCategory(ctx, groupID, suggestion.CategoryID, userID, row.Type); err != nil {
			continue
		}
		row.CategoryID = stringPointer(suggestion.CategoryID)
	}
	return nil
}

func scoreStatementMatch(row dao.ExpenseStatementIngestionRow, candidate statementMatchCandidate) float64 {
	return scoreStatementMatchWithMerchantKeys(row, candidate, normalizeMerchantKey(row.Merchant), normalizeMerchantKey(candidate.Merchant))
}

func scoreStatementMatchWithMerchantKeys(row dao.ExpenseStatementIngestionRow, candidate statementMatchCandidate, rowMerchant, candidateMerchant string) float64 {
	if row.OccurredOn == nil || row.Amount != candidate.Amount || row.Currency != candidate.Currency || row.Type != candidate.Type {
		return 0
	}
	rowDate, rowErr := time.Parse("2006-01-02", *row.OccurredOn)
	candidateDate, candidateErr := time.Parse("2006-01-02", candidate.OccurredOn)
	if rowErr != nil || candidateErr != nil {
		return 0
	}
	days := int(math.Abs(rowDate.Sub(candidateDate).Hours() / 24))
	if days > 3 {
		return 0
	}
	score := 0.50
	score += []float64{0.20, 0.15, 0.10, 0.05}[days]
	if rowMerchant != "" && rowMerchant == candidateMerchant {
		return math.Min(1, score+0.30)
	}
	if rowMerchant != "" && candidateMerchant != "" && (strings.Contains(rowMerchant, candidateMerchant) || strings.Contains(candidateMerchant, rowMerchant)) {
		return math.Min(1, score+0.22)
	}
	return math.Min(1, score+(0.30*statementTokenOverlap(rowMerchant, candidateMerchant)))
}

func statementMatchBucket(amount int, currency, entryType string) string {
	return fmt.Sprintf("%d|%s|%s", amount, currency, entryType)
}

func statementTokenOverlap(left, right string) float64 {
	leftTokens := strings.Fields(left)
	rightTokens := strings.Fields(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	leftSet := make(map[string]struct{}, len(leftTokens))
	union := make(map[string]struct{}, len(leftTokens)+len(rightTokens))
	for _, token := range leftTokens {
		leftSet[token] = struct{}{}
		union[token] = struct{}{}
	}
	intersection := 0
	for _, token := range rightTokens {
		if _, exists := leftSet[token]; exists {
			intersection++
		}
		union[token] = struct{}{}
	}
	return float64(intersection) / float64(len(union))
}

func buildStatementValidation(rows []dao.ExpenseStatementIngestionRow, currency string, previousBalance, declaredNewTransactionsTotal, statementGrandTotal *int, cardholderControls []map[string]any) map[string]any {
	unreadable := make([]string, 0)
	foreign := make([]string, 0)
	activity := 0
	newTransactionActivity := 0
	byCardholder := make(map[string][]dao.ExpenseStatementIngestionRow)
	for _, row := range rows {
		if row.OccurredOn == nil || strings.TrimSpace(row.Merchant) == "" || row.Amount == 0 {
			unreadable = append(unreadable, row.SourceRowKey)
		}
		if row.ForeignAmount != nil || strings.TrimSpace(row.ForeignCurrency) != "" {
			foreign = append(foreign, row.SourceRowKey)
		}
		// amount/currency are always the posted statement amounts. Foreign amount
		// and currency are display/provenance fields and must not remove a posted
		// SGD charge from the statement reconciliation.
		if row.Currency == currency {
			activity += signedStatementActivity(row)
			if row.StatementKind != "payment" {
				newTransactionActivity += signedStatementActivity(row)
				byCardholder[row.Cardholder] = append(byCardholder[row.Cardholder], row)
			}
		} else {
			unreadable = append(unreadable, row.SourceRowKey)
		}
	}
	result := map[string]any{"parsedRowCount": len(rows), "unreadableRows": unreadable, "foreignCurrencyRows": foreign, "signedTransactionActivity": activity, "signedNewTransactionActivity": newTransactionActivity}
	if declaredNewTransactionsTotal != nil {
		result["newTransactionsTotal"] = map[string]any{"expected": *declaredNewTransactionsTotal, "actual": newTransactionActivity, "delta": newTransactionActivity - *declaredNewTransactionsTotal, "pass": newTransactionActivity == *declaredNewTransactionsTotal}
	}
	if previousBalance != nil && statementGrandTotal != nil {
		expected := *previousBalance + activity
		result["balanceReconciliation"] = map[string]any{"previousBalance": *previousBalance, "signedActivity": activity, "expectedGrandTotal": expected, "actualGrandTotal": *statementGrandTotal, "delta": expected - *statementGrandTotal, "pass": expected == *statementGrandTotal}
	}
	cardholderChecks := make([]map[string]any, 0, len(cardholderControls))
	for _, declared := range cardholderControls {
		name, _ := declared["name"].(string)
		name = strings.TrimSpace(name)
		memberRows := byCardholder[name]
		subtotal := 0
		for _, row := range memberRows {
			subtotal += signedStatementActivity(row)
		}
		check := map[string]any{"name": name, "actualRowCount": len(memberRows), "actualSubtotal": subtotal}
		if declaredCount, ok := statementControlInt(declared["declaredRowCount"]); ok {
			check["declaredRowCount"] = declaredCount
			check["rowCountPass"] = declaredCount == len(memberRows)
		}
		if declaredSubtotal, ok := statementControlInt(declared["declaredSubtotal"]); ok {
			check["declaredSubtotal"] = declaredSubtotal
			check["subtotalPass"] = declaredSubtotal == subtotal
		}
		cardholderChecks = append(cardholderChecks, check)
	}
	if len(cardholderChecks) > 0 {
		result["cardholders"] = cardholderChecks
	}
	return result
}

func normalizeCardholderControls(values []request.StatementCardholderRequest) []map[string]any {
	controls := make([]map[string]any, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		control := map[string]any{"name": name}
		if value.DeclaredRowCount != nil {
			control["declaredRowCount"] = *value.DeclaredRowCount
		}
		if value.DeclaredSubtotal != nil {
			control["declaredSubtotal"] = *value.DeclaredSubtotal
		}
		controls = append(controls, control)
	}
	return controls
}

func statementControlInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), float64(int(typed)) == typed
	default:
		return 0, false
	}
}

func signedStatementActivity(row dao.ExpenseStatementIngestionRow) int {
	if row.Type == "income" {
		return -row.Amount
	}
	return row.Amount
}

func statementIngestionStatus(rows []dao.ExpenseStatementIngestionRow, changedID string, changed dao.ExpenseStatementIngestionRow) string {
	if len(rows) == 0 {
		return statementStatusParsed
	}
	for _, row := range rows {
		if row.ID == changedID {
			row = changed
		}
		if row.ReviewStatus == "unreviewed" {
			return statementStatusMatchingReview
		}
	}
	return statementStatusReady
}

func (s *ExpenseService) validateStatementRowForConfirmation(ctx context.Context, groupID, userID string, row dao.ExpenseStatementIngestionRow) error {
	if row.ReviewStatus == "ignored" {
		return nil
	}
	if row.ReviewStatus != "new" && row.ReviewStatus != "matched" {
		return fmt.Errorf("row has not been reviewed")
	}
	if row.OccurredOn == nil || strings.TrimSpace(*row.OccurredOn) == "" {
		return fmt.Errorf("occurredOn is required")
	}
	if strings.TrimSpace(row.Merchant) == "" {
		return fmt.Errorf("merchant is required")
	}
	if row.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if err := s.ensureActiveStatementAccount(ctx, groupID, derefString(row.AccountID)); err != nil {
		return err
	}
	if err := s.ensureActiveStatementCategory(ctx, groupID, derefString(row.CategoryID), userID, row.Type); err != nil {
		return err
	}
	if row.ReviewStatus == "matched" {
		return s.ensureMatchExpense(ctx, groupID, derefString(row.MatchExpenseID))
	}
	return nil
}

func (s *ExpenseService) ensureStatementGroupAccess(ctx context.Context, groupID, userID string) error {
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		return fmt.Errorf("group and authenticated user are required")
	}
	var id string
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select g.id::text from %s g join %s u on u.id = ?::uuid
		where g.id = ?::uuid and g.deleted_at is null and u.deleted_at is null
		and (u.group_id = g.id or g.created_by = u.id) limit 1
	`, dao.QualifiedTable("expense_groups"), dao.QualifiedTable("expense_users")), userID, groupID).Scan(&id).Error
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("group not found")
	}
	return nil
}

func (s *ExpenseService) ensureActiveStatementAccount(ctx context.Context, groupID, accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return fmt.Errorf("accountId is required")
	}
	var id string
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`select id::text from %s where id = ?::uuid and group_id = ?::uuid and deleted_at is null limit 1`, dao.QualifiedTable("expense_accounts")), accountID, groupID).Scan(&id).Error
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("account not found")
	}
	return nil
}

func (s *ExpenseService) ensureActiveStatementCategory(ctx context.Context, groupID, categoryID, userID, entryType string) error {
	if strings.TrimSpace(categoryID) == "" {
		return fmt.Errorf("categoryId is required")
	}
	category, err := s.findCategoryForUser(ctx, groupID, categoryID, userID)
	if err != nil {
		return err
	}
	if category.Type != entryType {
		return fmt.Errorf("category type mismatch: category is %s but entry is %s", category.Type, entryType)
	}
	return nil
}

func (s *ExpenseService) ensureMatchExpense(ctx context.Context, groupID, expenseID string) error {
	if strings.TrimSpace(expenseID) == "" {
		return fmt.Errorf("matchExpenseId is required")
	}
	var id string
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`select id::text from %s where id = ?::uuid and group_id = ?::uuid and deleted_at is null limit 1`, dao.QualifiedTable("expense_entries")), expenseID, groupID).Scan(&id).Error
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("matched transaction not found")
	}
	return nil
}

func (s *ExpenseService) ensureNoDuplicateStatementMatch(ctx context.Context, groupID, ingestionID, rowID, expenseID string) error {
	if strings.TrimSpace(expenseID) == "" {
		return fmt.Errorf("matchExpenseId is required")
	}
	var existingID string
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select id::text from %s
		where ingestion_id = ?::uuid and group_id = ?::uuid and match_expense_id = ?::uuid
		and id <> ?::uuid and deleted_at is null
		limit 1
	`, dao.QualifiedTable("expense_statement_ingestion_rows")), ingestionID, groupID, expenseID, rowID).Scan(&existingID).Error
	if err != nil {
		return err
	}
	if strings.TrimSpace(existingID) != "" {
		return fmt.Errorf("matched transaction is already assigned to another row in this ingestion")
	}
	return nil
}

func optionalStatementDate(raw string) (*string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := normalizeDate(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func normalizeStatementWarnings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 120 {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func findStatementIngestionRow(rows []dao.ExpenseStatementIngestionRow, rowID string) *dao.ExpenseStatementIngestionRow {
	for index := range rows {
		if rows[index].ID == strings.TrimSpace(rowID) {
			return &rows[index]
		}
	}
	return nil
}

func replaceStatementIngestionRow(rows []dao.ExpenseStatementIngestionRow, next dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
	result := append([]dao.ExpenseStatementIngestionRow(nil), rows...)
	for index := range result {
		if result[index].ID == next.ID {
			result[index] = next
			break
		}
	}
	return result
}

func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nilIfEmpty(value string) *string { return stringPointer(value) }
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
func derefStatementDate(value *string) string { return derefString(value) }
