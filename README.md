# Money Transfer Usecase - Backend Developer Assessment

A production-grade implementation of a money transfer usecase demonstrating proper architecture patterns, domain-driven design, and error handling.

## Architecture

This project follows a clean architecture pattern:

```
Service Layer (Applies mutations atomically)
    ↓
Usecase (Interactor) - Orchestrates business logic
    ↓
Domain (Business rules) - Account Withdraw/Deposit
    ↓
Repository (Persistence) - Returns mutations, never applies them
    ↓
Database (Commits mutations)
```

**Critical architecture rule**: Repositories RETURN mutations; they NEVER apply them. The service layer applies all mutations atomically.

## Project Structure

```
med/
├── contracts/                 # Interfaces and type definitions
│   └── repository.go         # Core interfaces and types
├── domain/                   # Business logic (domain entities)
│   └── account.go            # Account entity with Withdraw/Deposit
├── repo/                     # Repository implementation
│   └── account_repo.go       # AccountRepository with UpdateMut
├── usecases/transfer/        # Usecase implementation
│   ├── interactor.go         # TransferMoney usecase Execute()
│   └── interactor_test.go    # 13 comprehensive tests
├── REVIEW.md                 # Analysis of 10+ bugs in buggy code
├── ANSWERS.md                # Detailed Q&A on architecture patterns
├── go.mod                    # Go module definition
└── README.md                 # This file
```

## Key Features

### 1. Domain-Driven Design

The `Account` entity enforces business rules:

```go
// Business rules in domain layer
func (a *Account) Withdraw(amount int64) error {
    if amount <= 0 {
        return ErrInvalidAmount
    }
    if a.status != contracts.AccountStatusActive {
        return ErrAccountInactive
    }
    if a.balance < amount {
        return ErrInsufficientFunds
    }
    a.balance -= amount
    a.Changes.MarkDirty("balance")
    return nil
}
```

### 2. Mutation-Based Persistence

Repositories return mutations without applying them:

```go
// Repository returns mutation (doesn't apply)
func (r *AccountRepo) UpdateMut(account *Account) *Mutation {
    updates := make(map[string]interface{})
    if account.Changes.IsDirty("balance") {
        updates["balance"] = account.Balance()
    }
    // Only include changed fields
    return &Mutation{...}
}
```

### 3. Dirty Field Tracking

Only changed fields are included in mutations, preventing lost-update anomalies in concurrent scenarios:

```go
// Domain marks fields as changed
a.Changes.MarkDirty("balance")

// Repository only includes dirty fields
if account.Changes.IsDirty("balance") {
    updates["balance"] = account.Balance()
}
```

### 4. Comprehensive Error Handling

All inputs validated before domain logic:

```go
func (uc *Interactor) Execute(ctx context.Context, req *TransferRequest) (*Plan, error) {
    // 1. Validate request
    if req == nil || req.Amount <= 0 {
        return nil, ErrInvalidRequest
    }
    
    // 2. Retrieve both accounts
    source, err := uc.accountRepo.Retrieve(ctx, req.FromAccountID)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve source account: %w", err)
    }
    
    // 3. Call domain methods
    if err := sourceAccount.Withdraw(req.Amount); err != nil {
        return nil, err  // Domain error not wrapped
    }
    
    // 4. Get mutations from repository
    // 5. Return plan (don't apply)
}
```

## Running Tests

```bash
go test ./usecases/transfer -v
```

**Test Coverage** (13 tests):
- ✓ Happy path transfer
- ✓ Insufficient funds
- ✓ Source/destination account not found
- ✓ Same account transfer prevention
- ✓ Negative and zero amount validation
- ✓ Inactive source/destination accounts
- ✓ Only dirty fields in mutations
- ✓ Nil request handling
- ✓ Context cancellation
- ✓ Mutation structure validation

All tests pass ✓

## Architecture Violations in Buggy Code

The provided buggy code violates 7 architecture rules:

1. ❌ Ignores error returns (nil dereference risk)
2. ❌ Direct field access instead of domain methods
3. ❌ No input validation
4. ❌ **Applies mutations in usecase** (violates architecture)
5. ❌ Creates mutations directly instead of from repository
6. ❌ No dirty field tracking
7. ❌ Returns error instead of Plan
8. ❌ Partial failures create inconsistent state
9. ❌ Uses undefined `uc.db` interface
10. ❌ No context cancellation handling

**See REVIEW.md for detailed analysis of all 10 bugs.**

## Questions & Architecture Patterns

### Q1: Partial Failure Handling
When `source.Withdraw()` succeeds but `dest.Deposit()` fails:
- In-memory: source balance decreased, destination unchanged
- Database: **UNCHANGED** (no mutations in plan)
- Returned: `nil` plan and error
- Safe because: Plan not applied if any operation fails

**See ANSWERS.md Q1 for detailed state trace.**

### Q2: One-at-a-Time Application Problem
Applying mutations sequentially violates atomicity:
- If mutation1 succeeds and mutation2 fails: inconsistent state
- Example: Source withdraws but destination doesn't receive (money disappears)
- Solution: Apply all mutations in a single atomic database transaction

**See ANSWERS.md Q2 for failure scenario.**

### Q3: Dirty Fields in Concurrent Updates
Without dirty field tracking: lost update anomaly
- T1 (Admin): Changes status → FROZEN
- T2 (Transfer): Changes balance, includes all fields with stale status
- Result: Status reverts to ACTIVE (lost update)

With dirty fields:
- T1: Updates only status field
- T2: Updates only balance field
- Result: Both changes persist (no conflicts)

**See ANSWERS.md Q3 for detailed concurrent scenario.**

### Q4: Always-Include-All-Fields Problem
Including all fields creates write-write conflicts:
- Each mutation includes all fields with stale values
- Last write wins, overwriting other concurrent changes
- Example: Balance update includes old status, reverts freeze

Dirty field approach:
- Only changed fields in mutation
- Concurrent updates don't interfere
- Prevents lost updates

**See ANSWERS.md Q4 for real-world e-commerce example.**

## Implementation Highlights

### ✓ Correct Patterns Used

1. Domain methods enforce business rules
2. Repository returns mutations (never applies)
3. Usecase returns Plan with all mutations
4. Service layer applies Plan atomically
5. Only dirty fields included in mutations
6. Full input validation before domain logic
7. Proper error handling without panics
8. Context cancellation support

### ✓ Trade-offs & Design Decisions

- **Why return Plan not error**: Service layer needs to apply mutations atomically
- **Why dirty field tracking**: Prevents lost-update anomaly in concurrent operations
- **Why domain methods**: Enforce business rules at the right layer
- **Why repository doesn't apply**: Separates concerns, enables testing

## Further Reading

- **REVIEW.md** - Detailed bug analysis of the provided buggy code
- **ANSWERS.md** - In-depth Q&A on architecture patterns and concurrent scenarios
- **Clean Architecture** - Robert Martin's patterns applied here
- **Domain-Driven Design** - Eric Evans' patterns for domain logic

## Running the Code

```bash
# Set up Go environment
cd /home/boris/Desktop/med

# Run all tests
go test ./...

# Run specific test with output
go test ./usecases/transfer -v -run TestTransfer_HappyPath

# Run with coverage
go test ./... -cover
```

## Dependencies

- Go 1.21+
- No external dependencies (pure Go)

## Notes

This implementation demonstrates:
- Proper architecture layering
- Domain-driven design principles
- Concurrent-update safety through dirty field tracking
- Comprehensive error handling
- Full test coverage
- Production-grade code quality
