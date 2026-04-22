# Code Review: TransferMoney Usecase - Bug Analysis

## Critical Issues Found: 10 Bugs

### 1. **Ignoring Error Returns (Nil Dereference Risk)**
```go
source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)
dest, _ := uc.repo.Retrieve(ctx, req.ToAccountID)
```
**Problem**: Error returns are ignored with `_`. If either account doesn't exist, the variable will be `nil`, causing a **panic** when accessing `source.balance` or `dest.balance`.

**Impact**: Application crashes instead of graceful error handling.

**Correct Approach**: Check error and return early:
```go
source, err := uc.repo.Retrieve(ctx, req.FromAccountID)
if err != nil {
    return nil, fmt.Errorf("failed to retrieve source account: %w", err)
}
```

---

### 2. **Direct Field Manipulation - Bypasses Domain Logic**
```go
source.balance -= req.Amount
dest.balance += req.Amount
```
**Problem**: Domain methods (`Withdraw`, `Deposit`) are never called. This completely bypasses all business rule validation:
- No validation that balance won't go negative
- No validation that amount is positive  
- No validation that account is active
- No overflow checks
- No dirty field tracking

**Impact**: Business logic is entirely bypassed. Accounts can be corrupted into invalid states.

**Correct Approach**: Always use domain methods:
```go
if err := sourceAccount.Withdraw(req.Amount); err != nil {
    return nil, err
}
if err := destinationAccount.Deposit(req.Amount); err != nil {
    return nil, err
}
```

---

### 3. **No Request Validation**
```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error {
```
**Problem**: 
- No nil check on `req`
- No validation that `Amount > 0`
- No validation that `FromAccountID` and `ToAccountID` are non-empty
- No validation that `FromAccountID != ToAccountID`

**Impact**: Invalid transfers can silently succeed or cause panics.

**Correct Approach**: Validate all inputs at usecase entry point:
```go
if req == nil || req.Amount <= 0 || req.FromAccountID == req.ToAccountID {
    return nil, ErrInvalidRequest
}
```

---

### 4. **Violates Architecture: Mutations Applied by Usecase**
```go
if err := uc.db.Apply(mutation1); err != nil {
    return err
}
if err := uc.db.Apply(mutation2); err != nil {
    return err
}
```
**Problem**: The **architecture rule is violated**: "Repositories RETURN mutations, they NEVER apply them. The service layer applies all mutations atomically."

This code applies mutations directly in the usecase, which violates separation of concerns.

**Impact**: 
- Usecase couples to persistence concerns
- Usecase cannot be unit tested without a database
- Service layer cannot enforce atomic application
- Transaction boundaries are in the wrong layer

**Correct Approach**: 
- Return mutations in a `Plan`
- Let the service layer apply them atomically
- See our implementation: `return plan, nil`

---

### 5. **Mutations Created Directly Instead of From Repository**
```go
mutation1 := &Mutation{
    Table:   "accounts",
    ID:      string(source.id),
    Updates: map[string]interface{}{"balance": source.balance},
}
```
**Problem**: Mutations are created manually in the usecase instead of calling `uc.repo.UpdateMut()`. The repository should be the sole source of mutation creation - it knows about:
- Dirty field tracking
- What actually changed
- What fields can be updated
- How to serialize fields

**Impact**: 
- Repository pattern broken
- Repository has no control over what gets persisted
- Inconsistency between the actual field state and what's in the mutation

**Correct Approach**: Get mutations from repository:
```go
mutation1 := uc.repo.UpdateMut(sourceAccount)
plan.Add(mutation1)
```

---

### 6. **Includes All Fields Without Dirty Tracking**
```go
Updates: map[string]interface{}{"balance": source.balance},
```
**Problem**: No check for which fields actually changed. Even if only `balance` is included here, this hardcoded approach means:
- `status` field may or may not be included (inconsistent)
- No optimization for fields that didn't change
- With multiple fields, **lost-update** vulnerability

**Impact**: In concurrent scenarios, two simultaneous transfers can lose updates on status field changes.

**Correct Approach**: Only include fields that changed:
```go
// In repository
if account.Changes.IsDirty("balance") {
    updates["balance"] = account.Balance()
}
if account.Changes.IsDirty("status") {
    updates["status"] = account.Status()
}
```

---

### 7. **No Input Type for Return Value**
```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error {
```
**Problem**: The method returns `error` instead of `(*Plan, error)`. This means:
- Can't return the plan containing mutations
- Service layer can't apply mutations
- Caller doesn't know what would have been changed

**Impact**: Complete architecture violation - service layer can't work.

**Correct Approach**: 
```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*Plan, error) {
```

---

### 8. **Partial Failure - Inconsistent Database State**
```go
if err := uc.db.Apply(mutation1); err != nil {
    return err
}
// Source account applied!
if err := uc.db.Apply(mutation2); err != nil {
    return err  // Destination fails but source already changed!
}
```
**Problem**: If `mutation1` succeeds but `mutation2` fails:
- Source account balance decreased
- Destination account balance NOT increased
- Money disappears from the system (inconsistent transfer)
- No rollback mechanism

**Impact**: Data corruption. Money loss.

**Correct Approach**: Return mutations in a Plan and let service layer apply all atomically in a transaction.

---

### 9. **Using Unknown Interface - `uc.db`**
```go
if err := uc.db.Apply(mutation1); err != nil {
```
**Problem**: 
- `uc.db` is referenced but never defined in the function signature
- No interface contract for `db` / `Apply` method
- Unclear what `Apply` actually does
- Inconsistent with given `AccountRepository` interface

**Impact**: Unclear what this code even calls. No type safety.

**Correct Approach**: Use the well-defined `AccountRepository` interface and get mutations from it.

---

### 10. **No Context Cancellation Handling**
```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) error {
    source, _ := uc.repo.Retrieve(ctx, req.FromAccountID)
```
**Problem**: 
- The `ctx` parameter is passed to `Retrieve` (good)
- But the `Execute` method itself doesn't check if context was cancelled
- No way to cancel the operation between validation and apply

**Impact**: Cannot gracefully cancel in-flight operations.

**Correct Approach**: Repository will handle context, but usecase should propagate errors:
```go
source, err := uc.repo.Retrieve(ctx, req.FromAccountID)
if err != nil {
    // Repository will return ctx.Err() if context was cancelled
    return nil, fmt.Errorf("failed to retrieve source account: %w", err)
}
```

---

## Summary: Violations of Architecture Rules

| Rule | Violated? | Impact |
|------|-----------|--------|
| Repositories RETURN mutations | ✗ YES | Mutations created directly, architecture broken |
| Repositories NEVER apply mutations | ✗ YES | Apply called in usecase, not service layer |
| Service layer applies all mutations atomically | ✗ YES | No plan returned, no atomic application possible |
| Domain methods called (not direct field access) | ✗ YES | `balance -=` instead of `Withdraw()` |
| Only dirty fields in mutations | ✗ YES | No dirty field tracking |
| Validation before domain calls | ✗ YES | No input validation |
| Error handling without panic risk | ✗ YES | Produces nil dereference panics |

---

## What Our Implementation Does Correctly

1. ✓ Returns `(*Plan, error)` - service layer can apply Plan atomically
2. ✓ Calls domain methods `Withdraw()` and `Deposit()` - business logic applied
3. ✓ Gets mutations from repository via `UpdateMut()` - repository owns mutation creation
4. ✓ Only includes dirty fields - no unnecessary updates, no lost-update vulnerability
5. ✓ Full input validation - prevents invalid state
6. ✓ Proper error handling - no panics, clear error propagation
7. ✓ Dirty field tracking - `account.Changes.MarkDirty()` called by domain
