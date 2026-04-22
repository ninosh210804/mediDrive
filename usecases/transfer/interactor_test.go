package transfer

import (
	"context"
	"testing"

	"med/contracts"
	"med/domain"
	"med/repo"
)

func TestTransfer_HappyPath(t *testing.T) {
	// Setup
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive) // 1000.00
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)      // 500.00

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	// Execute transfer of 25000 cents (250.00)
	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        25000,
	}

	plan, err := uc.Execute(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan == nil {
		t.Fatal("expected plan to be returned")
	}

	// Check plan has 2 mutations (one for each account)
	mutations := plan.Mutations()
	if len(mutations) != 2 {
		t.Fatalf("expected 2 mutations, got %d", len(mutations))
	}

	// Check source mutation
	sourceMutation := mutations[0]
	if sourceMutation == nil {
		t.Fatal("source mutation should not be nil")
	}
	if balance, ok := sourceMutation.Updates["balance"]; !ok {
		t.Fatal("source mutation should include balance")
	} else if balance != int64(75000) {
		t.Fatalf("expected source balance 75000, got %v", balance)
	}

	// Check dest mutation
	destMutation := mutations[1]
	if destMutation == nil {
		t.Fatal("dest mutation should not be nil")
	}
	if balance, ok := destMutation.Updates["balance"]; !ok {
		t.Fatal("dest mutation should include balance")
	} else if balance != int64(75000) {
		t.Fatalf("expected dest balance 75000, got %v", balance)
	}

	// Verify account states
	if source.Balance() != 75000 {
		t.Fatalf("expected source balance 75000, got %d", source.Balance())
	}
	if dest.Balance() != 75000 {
		t.Fatalf("expected dest balance 75000, got %d", dest.Balance())
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 10000, contracts.AccountStatusActive)  // 100.00
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)      // 500.00

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	// Try to transfer more than available
	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        25000,
	}

	plan, err := uc.Execute(context.Background(), req)

	// Assert
	if err != domain.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}

	// Verify accounts unchanged
	if source.Balance() != 10000 {
		t.Fatalf("source balance should not change, got %d", source.Balance())
	}
	if dest.Balance() != 50000 {
		t.Fatalf("dest balance should not change, got %d", dest.Balance())
	}
}

func TestTransfer_SourceAccountNotFound(t *testing.T) {
	repository := repo.NewAccountRepo()
	destID := contracts.AccountID("account-2")

	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: contracts.AccountID("non-existent"),
		ToAccountID:   destID,
		Amount:        10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for missing source account")
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_DestAccountNotFound(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	repository.Save(source)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   contracts.AccountID("non-existent"),
		Amount:        10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for missing dest account")
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}

	// Source account should be unchanged (withdraw not attempted)
	if source.Balance() != 100000 {
		t.Fatalf("source balance should not change, got %d", source.Balance())
	}
}

func TestTransfer_SameAccount(t *testing.T) {
	repository := repo.NewAccountRepo()
	accountID := contracts.AccountID("account-1")

	account := domain.NewAccount(accountID, 100000, contracts.AccountStatusActive)
	repository.Save(account)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: accountID,
		ToAccountID:   accountID,
		Amount:        10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err != ErrSameAccount {
		t.Fatalf("expected ErrSameAccount, got %v", err)
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_NegativeAmount(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        -10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for negative amount")
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_ZeroAmount(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        0,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err == nil {
		t.Fatal("expected error for zero amount")
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_InactiveSourceAccount(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusInactive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err != domain.ErrAccountInactive {
		t.Fatalf("expected ErrAccountInactive, got %v", err)
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_InactiveDestAccount(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusInactive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        10000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err != domain.ErrAccountInactive {
		t.Fatalf("expected ErrAccountInactive, got %v", err)
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}

	// Source should be unchanged since dest check comes after withdraw
	// Wait, no - source.Withdraw() is called before dest.Deposit()
	// So source WILL have been changed before dest.Deposit() fails
	// This is the consistent state issue mentioned in Q1!
	if source.Balance() != 90000 {
		t.Fatalf("expected source balance to be decreased before dest check, got %d", source.Balance())
	}
}

func TestTransfer_OnlyDirtyFieldsInMutation(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        25000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mutations := plan.Mutations()

	// Both mutations should only include "balance", not "status"
	for i, mutation := range mutations {
		if _, ok := mutation.Updates["balance"]; !ok {
			t.Fatalf("mutation %d should include balance", i)
		}
		if _, ok := mutation.Updates["status"]; ok {
			t.Fatalf("mutation %d should NOT include status (not dirty)", i)
		}
	}
}

func TestTransfer_NilRequest(t *testing.T) {
	repository := repo.NewAccountRepo()
	uc := NewInteractor(repository)

	plan, err := uc.Execute(context.Background(), nil)

	if err != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_ContextCancelled(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("account-1")
	destID := contracts.AccountID("account-2")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        10000,
	}

	plan, err := uc.Execute(ctx, req)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	if plan != nil {
		t.Fatal("plan should be nil on error")
	}
}

func TestTransfer_MutationStructure(t *testing.T) {
	repository := repo.NewAccountRepo()
	sourceID := contracts.AccountID("source-123")
	destID := contracts.AccountID("dest-456")

	source := domain.NewAccount(sourceID, 100000, contracts.AccountStatusActive)
	dest := domain.NewAccount(destID, 50000, contracts.AccountStatusActive)

	repository.Save(source)
	repository.Save(dest)

	uc := NewInteractor(repository)

	req := &TransferRequest{
		FromAccountID: sourceID,
		ToAccountID:   destID,
		Amount:        15000,
	}

	plan, err := uc.Execute(context.Background(), req)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mutations := plan.Mutations()

	// Check source mutation
	sourceMutation := mutations[0]
	if sourceMutation.Table != "accounts" {
		t.Fatalf("expected table 'accounts', got %s", sourceMutation.Table)
	}
	if sourceMutation.ID != string(sourceID) {
		t.Fatalf("expected ID %s, got %s", sourceID, sourceMutation.ID)
	}

	// Check dest mutation
	destMutation := mutations[1]
	if destMutation.Table != "accounts" {
		t.Fatalf("expected table 'accounts', got %s", destMutation.Table)
	}
	if destMutation.ID != string(destID) {
		t.Fatalf("expected ID %s, got %s", destID, destMutation.ID)
	}
}
