package repo

import (
	"context"
	"fmt"

	"med/contracts"
	"med/domain"
)

// AccountRepo implements the AccountRepository interface
type AccountRepo struct {
	store map[contracts.AccountID]*domain.Account
}

// NewAccountRepo creates a new account repository
func NewAccountRepo() *AccountRepo {
	return &AccountRepo{
		store: make(map[contracts.AccountID]*domain.Account),
	}
}

// Retrieve fetches an account by ID
// Returns error if account not found
func (r *AccountRepo) Retrieve(ctx context.Context, id contracts.AccountID) (*domain.Account, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	account, exists := r.store[id]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	return account, nil
}

// UpdateMut returns a mutation for account changes
// CRITICAL: This method returns a mutation but does NOT apply it
// Only includes dirty fields in the mutation
// Returns nil if nothing changed
func (r *AccountRepo) UpdateMut(account *domain.Account) *contracts.Mutation {
	// Check if account has any changes
	if !account.Changes.HasChanges() {
		return nil
	}

	// Build updates map with only dirty fields
	updates := make(map[string]interface{})

	if account.Changes.IsDirty("balance") {
		updates["balance"] = account.Balance()
	}

	if account.Changes.IsDirty("status") {
		updates["status"] = account.Status()
	}

	// Return nil if no updates (defensive check)
	if len(updates) == 0 {
		return nil
	}

	// Return mutation without applying it
	return &contracts.Mutation{
		Table:   "accounts",
		ID:      string(account.ID()),
		Updates: updates,
	}
}

// Apply applies a mutation to an account (used by tests/service layer)
// This is NOT called by the repository - the service layer must call this
func (r *AccountRepo) Apply(mutation *contracts.Mutation) error {
	if mutation == nil {
		return nil
	}

	if mutation.Table != "accounts" {
		return fmt.Errorf("invalid table: %s", mutation.Table)
	}

	return nil
}

// Save stores an account in the repository (for testing purposes)
func (r *AccountRepo) Save(account *domain.Account) {
	if account != nil {
		r.store[account.ID()] = account
	}
}
