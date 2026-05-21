package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/gorm"
)

type ExpenseService struct {
	DB *gorm.DB
}

func NewExpenseService(db *gorm.DB) *ExpenseService {
	return &ExpenseService{DB: db}
}

func (s *ExpenseService) ResolveOrCreateUserGroup(ctx context.Context, userID string) (*dao.ExpenseGroup, error) {
	owner := strings.TrimSpace(userID)
	if owner == "" {
		return nil, fmt.Errorf("user is required")
	}

	group := &dao.ExpenseGroup{}
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			name,
			invite_code,
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from %s
		where created_by = ?::uuid and deleted_at is null
		order by updated_at desc
		limit 1
	`, dao.QualifiedTable("expense_groups")), owner).Scan(group).Error
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(group.ID) != "" {
		return group, nil
	}

	return s.CreateGroup(ctx, &request.CreateGroupRequest{
		Name:      "Household",
		CreatedBy: owner,
	})
}

func (s *ExpenseService) CreateGroup(ctx context.Context, req *request.CreateGroupRequest) (*dao.ExpenseGroup, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate group id: %w", err)
	}

	now := time.Now().UTC()
	inviteCode := strings.ToUpper(strings.ReplaceAll(id, "-", ""))[:6]
	var createdBy *string
	if trimmed := strings.TrimSpace(req.CreatedBy); trimmed != "" {
		createdBy = &trimmed
	}

	group := &dao.ExpenseGroup{
		ID:         id,
		Name:       name,
		InviteCode: inviteCode,
		CreatedBy:  createdBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.DB.WithContext(ctx).Table((dao.ExpenseGroup{}).TableName()).Create(group).Error; err != nil {
		return nil, err
	}

	return group, nil
}

func (s *ExpenseService) CreateAccount(ctx context.Context, groupID string, req *request.CreateAccountRequest) (*dao.ExpenseAccount, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	accountType := strings.ToLower(strings.TrimSpace(req.Type))
	if accountType == "" {
		return nil, fmt.Errorf("type is required")
	}

	switch accountType {
	case "cash", "bank", "card", "wallet":
	default:
		return nil, fmt.Errorf("type must be one of cash, bank, card, wallet")
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate account id: %w", err)
	}

	now := time.Now().UTC()
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#4b5745"
	}
	icon := normalizeIcon(req.Icon, defaultAccountIcon(accountType))

	account := &dao.ExpenseAccount{
		ID:             id,
		GroupID:        groupID,
		Name:           name,
		Type:           accountType,
		OpeningBalance: req.OpeningBalance,
		Color:          color,
		Icon:           icon,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.DB.WithContext(ctx).Table((dao.ExpenseAccount{}).TableName()).Create(account).Error; err != nil {
		return nil, err
	}

	return account, nil
}

func (s *ExpenseService) ListAccounts(ctx context.Context, groupID string) ([]dao.ExpenseAccount, error) {
	records := make([]dao.ExpenseAccount, 0)
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			opening_balance,
			color,
			icon,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and deleted_at is null
		order by updated_at desc
	`, dao.QualifiedTable("expense_accounts")), groupID).Scan(&records).Error
	return records, err
}

func (s *ExpenseService) CreateCategory(ctx context.Context, groupID string, authUserID string, req *request.CreateCategoryRequest) (*dao.ExpenseCategory, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate category id: %w", err)
	}

	now := time.Now().UTC()
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#e7d24e"
	}
	categoryType := normalizeCategoryType(req.Type, name)
	categoryScope := normalizeCategoryScope(req.Scope)
	icon := normalizeIcon(req.Icon, defaultCategoryIcon(name))
	var ownerUserID *string
	if categoryScope == "user" {
		trimmedUserID := strings.TrimSpace(authUserID)
		if trimmedUserID == "" {
			return nil, fmt.Errorf("authenticated user is required for user-level category")
		}
		ownerUserID = &trimmedUserID
	}

	category := &dao.ExpenseCategory{
		ID:            id,
		GroupID:       groupID,
		Name:          name,
		Type:          categoryType,
		Scope:         categoryScope,
		OwnerUserID:   ownerUserID,
		Color:         color,
		Icon:          icon,
		MonthlyTarget: req.MonthlyTarget,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.DB.WithContext(ctx).Table((dao.ExpenseCategory{}).TableName()).Create(category).Error; err != nil {
		return nil, err
	}

	return category, nil
}

func (s *ExpenseService) ListCategories(ctx context.Context, groupID string, authUserID string) ([]dao.ExpenseCategory, error) {
	records := make([]dao.ExpenseCategory, 0)
	authUserID = strings.TrimSpace(authUserID)
	accessClause := "(scope = 'household')"
	args := []any{groupID}
	if authUserID != "" {
		accessClause = "(scope = 'household' or (scope = 'user' and owner_user_id = ?::uuid))"
		args = append(args, authUserID)
	}

	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			scope,
			owner_user_id::text as owner_user_id,
			color,
			icon,
			monthly_target,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and deleted_at is null and %s
		order by updated_at desc
	`, dao.QualifiedTable("expense_categories"), accessClause), args...).Scan(&records).Error
	return records, err
}

func (s *ExpenseService) CreateAutomationEntry(ctx context.Context, authUserID string, req *request.CreateAutomationEntryRequest) (*dao.ExpenseEntry, error) {
	trimmedUserID := strings.TrimSpace(authUserID)
	if trimmedUserID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	accountRef := strings.TrimSpace(req.AccountType)
	if accountRef == "" {
		return nil, fmt.Errorf("accountType is required")
	}

	merchant := strings.TrimSpace(req.Merchant)
	if merchant == "" {
		return nil, fmt.Errorf("merchant is required")
	}

	group, err := s.ResolveOrCreateUserGroup(ctx, trimmedUserID)
	if err != nil {
		return nil, err
	}

	account, err := s.findOrCreateAccountByRef(ctx, group.ID, accountRef)
	if err != nil {
		return nil, err
	}

	entryType := "expense"
	amount := req.Amount
	if amount < 0 {
		amount = -amount
	}

	category, err := s.findOrCreateHouseholdCategoryByType(ctx, group.ID, trimmedUserID, entryType)
	if err != nil {
		return nil, err
	}

	occurredOn, err := normalizeCreatedAtDate(req.CreatedAt)
	if err != nil {
		return nil, err
	}

	return s.CreateExpense(ctx, group.ID, trimmedUserID, &request.CreateExpenseRequest{
		AccountID:  account.ID,
		CategoryID: category.ID,
		Type:       entryType,
		Amount:     amount,
		Currency:   "SGD",
		OccurredOn: occurredOn,
		Merchant:   merchant,
		Note:       "",
	})
}

func (s *ExpenseService) CreateExpense(ctx context.Context, groupID string, createdByUserID string, req *request.CreateExpenseRequest) (*dao.ExpenseEntry, error) {
	if strings.TrimSpace(req.AccountID) == "" {
		return nil, fmt.Errorf("accountId is required")
	}
	if strings.TrimSpace(req.CategoryID) == "" {
		return nil, fmt.Errorf("categoryId is required")
	}
	if req.Amount < 0 {
		return nil, fmt.Errorf("amount must be >= 0")
	}

	entryType := strings.ToLower(strings.TrimSpace(req.Type))
	if entryType == "" {
		entryType = "expense"
	}
	if entryType != "expense" && entryType != "income" {
		return nil, fmt.Errorf("type must be expense or income")
	}

	categoryMeta, err := s.findCategoryForUser(ctx, groupID, strings.TrimSpace(req.CategoryID), createdByUserID)
	if err != nil {
		return nil, err
	}
	if categoryMeta.Type != "" && categoryMeta.Type != entryType {
		return nil, fmt.Errorf("category type mismatch: category is %s but entry is %s", categoryMeta.Type, entryType)
	}

	merchant := strings.TrimSpace(req.Merchant)
	if merchant == "" {
		return nil, fmt.Errorf("merchant is required")
	}

	occurredOn, err := normalizeDate(req.OccurredOn)
	if err != nil {
		return nil, err
	}
	currencyCode, err := normalizeCurrencyCode(req.Currency)
	if err != nil {
		return nil, err
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate expense id: %w", err)
	}

	now := time.Now().UTC()
	var createdBy *string
	if trimmed := strings.TrimSpace(createdByUserID); trimmed != "" {
		createdBy = &trimmed
	}

	expense := &dao.ExpenseEntry{
		ID:         id,
		GroupID:    groupID,
		AccountID:  strings.TrimSpace(req.AccountID),
		CategoryID: strings.TrimSpace(req.CategoryID),
		Type:       entryType,
		Amount:     req.Amount,
		Currency:   currencyCode,
		OccurredOn: occurredOn,
		Merchant:   merchant,
		Note:       strings.TrimSpace(req.Note),
		CreatedBy:  createdBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table((dao.ExpenseEntry{}).TableName()).Create(expense).Error; err != nil {
			return err
		}
		return upsertMerchant(tx, merchantFromEntry(groupID, *expense, now))
	})
	if err != nil {
		return nil, err
	}

	return expense, nil
}

func (s *ExpenseService) UpdateExpense(ctx context.Context, groupID string, transactionID string, authUserID string, req *request.UpdateExpenseRequest) (*dao.ExpenseEntry, error) {
	trimmedTransactionID := strings.TrimSpace(transactionID)
	if trimmedTransactionID == "" {
		return nil, fmt.Errorf("transactionId is required")
	}

	current := &dao.ExpenseEntry{}
	if err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			account_id::text as account_id,
			category_id::text as category_id,
			type,
			amount,
			currency,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
			merchant,
			note,
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from %s
		where id = ?::uuid and group_id = ?::uuid and deleted_at is null
		limit 1
	`, dao.QualifiedTable("expense_entries")), trimmedTransactionID, groupID).Scan(current).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(current.ID) == "" {
		return nil, fmt.Errorf("transaction not found")
	}

	next := *current

	if req.AccountID != nil {
		nextAccountID := strings.TrimSpace(*req.AccountID)
		if nextAccountID == "" {
			return nil, fmt.Errorf("accountId is required")
		}
		next.AccountID = nextAccountID
	}

	if req.CategoryID != nil {
		nextCategoryID := strings.TrimSpace(*req.CategoryID)
		if nextCategoryID == "" {
			return nil, fmt.Errorf("categoryId is required")
		}
		next.CategoryID = nextCategoryID
	}

	if req.Type != nil {
		nextType := strings.ToLower(strings.TrimSpace(*req.Type))
		if nextType == "" {
			nextType = "expense"
		}
		if nextType != "expense" && nextType != "income" {
			return nil, fmt.Errorf("type must be expense or income")
		}
		next.Type = nextType
	}

	if req.Amount != nil {
		if *req.Amount < 0 {
			return nil, fmt.Errorf("amount must be >= 0")
		}
		next.Amount = *req.Amount
	}

	if req.Currency != nil {
		currencyCode, err := normalizeCurrencyCode(*req.Currency)
		if err != nil {
			return nil, err
		}
		next.Currency = currencyCode
	}

	if req.OccurredOn != nil {
		trimmedDate := strings.TrimSpace(*req.OccurredOn)
		if trimmedDate != "" {
			occurredOn, err := normalizeDate(trimmedDate)
			if err != nil {
				return nil, err
			}
			next.OccurredOn = occurredOn
		}
	}

	if req.Merchant != nil {
		merchant := strings.TrimSpace(*req.Merchant)
		if merchant == "" {
			return nil, fmt.Errorf("merchant is required")
		}
		next.Merchant = merchant
	}

	if req.Note != nil {
		next.Note = strings.TrimSpace(*req.Note)
	}

	categoryMeta, err := s.findCategoryForUser(ctx, groupID, strings.TrimSpace(next.CategoryID), authUserID)
	if err != nil {
		return nil, err
	}
	if categoryMeta.Type != "" && categoryMeta.Type != next.Type {
		return nil, fmt.Errorf("category type mismatch: category is %s but entry is %s", categoryMeta.Type, next.Type)
	}

	now := time.Now().UTC()
	next.UpdatedAt = now

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table((dao.ExpenseEntry{}).TableName()).
			Where("id = ?::uuid and group_id = ?::uuid and deleted_at is null", trimmedTransactionID, groupID).
			Updates(map[string]any{
				"account_id":  next.AccountID,
				"category_id": next.CategoryID,
				"type":        next.Type,
				"amount":      next.Amount,
				"currency":    next.Currency,
				"occurred_on": next.OccurredOn,
				"merchant":    next.Merchant,
				"note":        next.Note,
				"updated_at":  next.UpdatedAt,
			}).Error; err != nil {
			return err
		}

		return upsertMerchant(tx, merchantFromEntry(groupID, next, now))
	})
	if err != nil {
		return nil, err
	}

	return &next, nil
}

func (s *ExpenseService) ListExpenses(ctx context.Context, groupID string, req *request.ListExpensesRequest) ([]dao.ExpenseEntry, error) {
	records := make([]dao.ExpenseEntry, 0)

	query := s.DB.WithContext(ctx).Table(fmt.Sprintf("%s as e", dao.QualifiedTable("expense_entries"))).Select(`
		e.id::text as id,
		e.group_id::text as group_id,
		e.account_id::text as account_id,
		e.category_id::text as category_id,
		e.type,
		e.amount,
		e.currency,
		to_char(e.occurred_on, 'YYYY-MM-DD') as occurred_on,
		e.merchant,
		e.note,
		e.created_by::text as created_by,
		e.created_at,
		e.updated_at,
		e.deleted_at
	`).Where("e.group_id = ?::uuid and e.deleted_at is null", groupID)

	if req != nil {
		entryType := strings.ToLower(strings.TrimSpace(req.Type))
		if entryType != "" {
			if entryType != "expense" && entryType != "income" {
				return nil, fmt.Errorf("type must be expense or income")
			}
			query = query.Where("e.type = ?", entryType)
		}

		if req.MonthsBack > 0 {
			cutoff := time.Now().UTC().AddDate(0, -req.MonthsBack, 0).Format("2006-01-02")
			query = query.Where("e.occurred_on >= ?::date", cutoff)
		}

		keyword := strings.ToLower(strings.TrimSpace(req.Query))
		if keyword != "" {
			pattern := "%" + keyword + "%"
			query = query.Where(fmt.Sprintf(`
				lower(e.merchant) like ? or
				lower(e.note) like ? or
				lower(e.type) like ? or
				lower(e.currency) like ? or
				to_char(e.occurred_on, 'YYYY-MM-DD') like ? or
				cast(e.amount as text) like ? or
				exists (
					select 1
					from %s a
					where a.id = e.account_id and a.group_id = e.group_id and a.deleted_at is null and lower(a.name) like ?
				) or
				exists (
					select 1
					from %s c
					where c.id = e.category_id and c.group_id = e.group_id and c.deleted_at is null and lower(c.name) like ?
				)
			`,
				dao.QualifiedTable("expense_accounts"),
				dao.QualifiedTable("expense_categories")),
				pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
		}

		limit := req.Limit
		if limit > 0 {
			if limit > 1000 {
				limit = 1000
			}
			query = query.Limit(limit)
		}
	}

	err := query.Order("e.occurred_on desc, e.updated_at desc").Scan(&records).Error
	return records, err
}

func (s *ExpenseService) CreateAdjustment(ctx context.Context, groupID string, authUserID string, req *request.CreateAdjustmentRequest) (*dao.ExpenseCategoryAdjustment, error) {
	if strings.TrimSpace(req.CategoryID) == "" {
		return nil, fmt.Errorf("categoryId is required")
	}
	if _, err := s.findCategoryForUser(ctx, groupID, strings.TrimSpace(req.CategoryID), authUserID); err != nil {
		return nil, err
	}

	occurredOn, err := normalizeDate(req.OccurredOn)
	if err != nil {
		return nil, err
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate adjustment id: %w", err)
	}

	now := time.Now().UTC()
	adjustment := &dao.ExpenseCategoryAdjustment{
		ID:         id,
		GroupID:    groupID,
		CategoryID: strings.TrimSpace(req.CategoryID),
		Amount:     req.Amount,
		OccurredOn: occurredOn,
		Note:       strings.TrimSpace(req.Note),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.DB.WithContext(ctx).Table((dao.ExpenseCategoryAdjustment{}).TableName()).Create(adjustment).Error; err != nil {
		return nil, err
	}

	return adjustment, nil
}

func (s *ExpenseService) ListAdjustments(ctx context.Context, groupID string) ([]dao.ExpenseCategoryAdjustment, error) {
	records := make([]dao.ExpenseCategoryAdjustment, 0)
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			category_id::text as category_id,
			amount,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
			note,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and deleted_at is null
		order by occurred_on desc, updated_at desc
	`, dao.QualifiedTable("expense_category_adjustments")), groupID).Scan(&records).Error
	return records, err
}

func normalizeDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", fmt.Errorf("occurredOn must be YYYY-MM-DD")
	}
	return parsed.Format("2006-01-02"), nil
}

func normalizeCreatedAtDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC().Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("createdAt must be RFC3339 or YYYY-MM-DD")
}

func normalizeCurrencyCode(value string) (string, error) {
	_ = strings.TrimSpace(value)
	return "SGD", nil
}

func normalizeIcon(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

type categoryMeta struct {
	ID          string
	Name        string
	Type        string
	Scope       string
	OwnerUserID *string
}

func (s *ExpenseService) findCategoryForUser(ctx context.Context, groupID string, categoryID string, authUserID string) (*categoryMeta, error) {
	var category categoryMeta
	if err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			name,
			type,
			scope,
			owner_user_id::text as owner_user_id
		from %s
		where id = ?::uuid and group_id = ?::uuid and deleted_at is null
		limit 1
	`, dao.QualifiedTable("expense_categories")), categoryID, groupID).Scan(&category).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(category.ID) == "" {
		return nil, fmt.Errorf("category not found")
	}

	category.Scope = normalizeCategoryScope(category.Scope)
	category.Type = normalizeCategoryType(category.Type, category.Name)
	if category.Scope == "user" {
		trimmedUserID := strings.TrimSpace(authUserID)
		if trimmedUserID == "" {
			return nil, fmt.Errorf("unauthorized category access")
		}
		if category.OwnerUserID == nil || strings.TrimSpace(*category.OwnerUserID) == "" {
			return nil, fmt.Errorf("invalid user category owner")
		}
		if strings.TrimSpace(*category.OwnerUserID) != trimmedUserID {
			return nil, fmt.Errorf("category does not belong to current user")
		}
	}

	return &category, nil
}

func defaultAccountIcon(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "cash":
		return "💵"
	case "card":
		return "💳"
	case "wallet":
		return "👛"
	case "bank":
		fallthrough
	default:
		return "🏦"
	}
}

func defaultCategoryIcon(categoryName string) string {
	normalized := strings.ToLower(strings.TrimSpace(categoryName))
	switch {
	case strings.Contains(normalized, "grocer"), strings.Contains(normalized, "food"), strings.Contains(normalized, "eat"):
		return "🍽️"
	case strings.Contains(normalized, "transport"), strings.Contains(normalized, "fuel"), strings.Contains(normalized, "car"):
		return "🚗"
	case strings.Contains(normalized, "home"), strings.Contains(normalized, "rent"):
		return "🏠"
	case strings.Contains(normalized, "health"), strings.Contains(normalized, "medic"):
		return "🩺"
	case strings.Contains(normalized, "income"), strings.Contains(normalized, "salary"):
		return "💼"
	default:
		return "🏷️"
	}
}

func normalizeCategoryType(value string, nameFallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "income":
		return "income"
	case "expense":
		return "expense"
	}

	normalized := strings.ToLower(strings.TrimSpace(nameFallback))
	if strings.Contains(normalized, "income") || strings.Contains(normalized, "salary") || strings.Contains(normalized, "payroll") {
		return "income"
	}
	return "expense"
}

func normalizeCategoryScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "user"
	default:
		return "household"
	}
}

func (s *ExpenseService) findOrCreateAccountByRef(ctx context.Context, groupID string, accountRef string) (*dao.ExpenseAccount, error) {
	trimmedRef := strings.TrimSpace(accountRef)
	if trimmedRef == "" {
		return nil, fmt.Errorf("accountType is required")
	}

	refLower := strings.ToLower(trimmedRef)
	account := &dao.ExpenseAccount{}
	if err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			opening_balance,
			color,
			icon,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and deleted_at is null and (lower(name) = ? or lower(type) = ?)
		order by updated_at desc
		limit 1
	`, dao.QualifiedTable("expense_accounts")), groupID, refLower, refLower).Scan(account).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(account.ID) != "" {
		return account, nil
	}

	derivedType := normalizeAccountTypeFromRef(trimmedRef)
	created, err := s.CreateAccount(ctx, groupID, &request.CreateAccountRequest{
		Name:           trimmedRef,
		Type:           derivedType,
		OpeningBalance: 0,
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *ExpenseService) findOrCreateHouseholdCategoryByType(ctx context.Context, groupID string, authUserID string, entryType string) (*dao.ExpenseCategory, error) {
	normalizedType := normalizeCategoryType(entryType, "")

	category := &dao.ExpenseCategory{}
	if err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			scope,
			owner_user_id::text as owner_user_id,
			color,
			icon,
			monthly_target,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and type = ? and scope = 'household' and deleted_at is null
		order by updated_at desc
		limit 1
	`, dao.QualifiedTable("expense_categories")), groupID, normalizedType).Scan(category).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(category.ID) != "" {
		return category, nil
	}

	categoryName := "Other expense"
	if normalizedType == "income" {
		categoryName = "Income"
	}

	created, err := s.CreateCategory(ctx, groupID, authUserID, &request.CreateCategoryRequest{
		Name:          categoryName,
		Type:          normalizedType,
		Scope:         "household",
		MonthlyTarget: 0,
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func normalizeAccountTypeFromRef(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "cash", "bank", "card", "wallet":
		return normalized
	}

	switch {
	case strings.Contains(normalized, "cash"):
		return "cash"
	case strings.Contains(normalized, "card"), strings.Contains(normalized, "visa"), strings.Contains(normalized, "master"), strings.Contains(normalized, "amex"):
		return "card"
	case strings.Contains(normalized, "bank"):
		return "bank"
	default:
		return "wallet"
	}
}
