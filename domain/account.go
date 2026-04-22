package domain

import (
	"errors"
	"fmt"

	"med/contracts"
)

var (
	// ErrInsufficientFunds occurs when attempting to withdraw more than available
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrInvalidAmount occurs when amount is zero or negative
	ErrInvalidAmount = errors.New("invalid amount: must be positive")
	// ErrAccountInactive occurs when account is not active
	ErrAccountInactive = errors.New("account is not active")
	// ErrNegativeBalance occurs when balance would go negative
	ErrNegativeBalance = errors.New("operation would result in negative balance")
)

// Account represents a bank account in the domain
type Account struct {
	id      contracts.AccountID
	balance int64 // in cents
	status  contracts.AccountStatus
	Changes *contracts.ChangeTracker
}

// NewAccount creates a new account with the given parameters
func NewAccount(id contracts.AccountID, initialBalance int64, status contracts.AccountStatus) *Account {
	return &Account{
		id:      id,
		balance: initialBalance,
		status:  status,
		Changes: contracts.NewChangeTracker(),
	}
}

// ID returns the account ID
func (a *Account) ID() contracts.AccountID {
	return a.id
}

// Balance returns the current balance in cents
func (a *Account) Balance() int64 {
	return a.balance
}

// Status returns the account status
func (a *Account) Status() contracts.AccountStatus {
	return a.status
}

// Withdraw withdraws an amount from the account
// This is a domain method that enforces business rules
func (a *Account) Withdraw(amount int64) error {
	// Validate amount
	if amount <= 0 {
		return ErrInvalidAmount
	}

	// Check account is active
	if a.status != contracts.AccountStatusActive {
		return ErrAccountInactive
	}

	// Check sufficient funds
	if a.balance < amount {
		return ErrInsufficientFunds
	}

	// Apply the withdrawal
	a.balance -= amount
	a.Changes.MarkDirty("balance")

	return nil
}

// Deposit deposits an amount to the account
// This is a domain method that enforces business rules
func (a *Account) Deposit(amount int64) error {
	// Validate amount
	if amount <= 0 {
		return ErrInvalidAmount
	}

	// Check account is active
	if a.status != contracts.AccountStatusActive {
		return ErrAccountInactive
	}

	// Check for overflow
	if a.balance > 0 && amount > (1<<63-1)-a.balance {
		return fmt.Errorf("deposit would cause overflow")
	}

	// Apply the deposit
	a.balance += amount
	a.Changes.MarkDirty("balance")

	return nil
}

// SetStatus sets the account status (for authorized operations)
func (a *Account) SetStatus(status contracts.AccountStatus) {
	if a.status != status {
		a.status = status
		a.Changes.MarkDirty("status")
	}
}
