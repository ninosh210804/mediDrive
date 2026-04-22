package transfer

import (
	"context"
	"errors"
	"fmt"

	"med/contracts"
	"med/repo"
)

var (
	ErrInvalidRequest = errors.New("invalid transfer request")
	ErrSameAccount    = errors.New("cannot transfer to the same account")
	ErrTransferFailed = errors.New("transfer failed")
)

// TransferRequest is the input for the transfer usecase
type TransferRequest struct {
	FromAccountID contracts.AccountID
	ToAccountID   contracts.AccountID
	Amount        int64
}

// Interactor implements the transfer usecase
type Interactor struct {
	accountRepo *repo.AccountRepo
}

// NewInteractor creates a new transfer interactor
func NewInteractor(accountRepo *repo.AccountRepo) *Interactor {
	return &Interactor{
		accountRepo: accountRepo,
	}
}

// Execute executes the transfer usecase
// Returns a Plan containing all mutations to be applied atomically
// CRITICAL: This method does NOT apply mutations - it only collects them into a Plan
// The service layer is responsible for applying the plan atomically
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*contracts.Plan, error) {
	// 1. VALIDATE REQUEST
	if req == nil {
		return nil, ErrInvalidRequest
	}

	if req.Amount <= 0 {
		return nil, fmt.Errorf("invalid amount: must be positive")
	}

	if req.FromAccountID == "" || req.ToAccountID == "" {
		return nil, ErrInvalidRequest
	}

	if req.FromAccountID == req.ToAccountID {
		return nil, ErrSameAccount
	}

	// 2. RETRIEVE BOTH ACCOUNTS
	sourceAccount, err := uc.accountRepo.Retrieve(ctx, req.FromAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve source account: %w", err)
	}

	destinationAccount, err := uc.accountRepo.Retrieve(ctx, req.ToAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve destination account: %w", err)
	}

	// 3. CALL DOMAIN METHODS (DO NOT ACCESS FIELDS DIRECTLY)
	// Withdraw from source - this applies business rules and marks changes
	if err := sourceAccount.Withdraw(req.Amount); err != nil {
		// Domain error - return as-is without wrapping
		return nil, err
	}

	// Deposit to destination - this applies business rules and marks changes
	if err := destinationAccount.Deposit(req.Amount); err != nil {
		// Domain error - return as-is without wrapping
		// Note: In a transaction context, the source withdrawal would be rolled back
		// For this usecase, we return the error and expect the caller to handle rollback
		return nil, err
	}

	// 4. GET MUTATIONS FROM REPOSITORY (NOT CREATED DIRECTLY)
	plan := contracts.NewPlan()

	// Get mutation for source account (only includes dirty fields)
	sourceMutation := uc.accountRepo.UpdateMut(sourceAccount)
	plan.Add(sourceMutation)

	// Get mutation for destination account (only includes dirty fields)
	destMutation := uc.accountRepo.UpdateMut(destinationAccount)
	plan.Add(destMutation)

	// 5. RETURN PLAN (DO NOT APPLY)
	// The service layer is responsible for applying mutations atomically
	return plan, nil
}
