# Technical Questions - Detailed Answers

## Q1: State Consistency on Partial Failure

**Question**: In your implementation, what happens if `source.Withdraw()` succeeds but `dest.Deposit()` fails? Show the exact state of both accounts and the returned plan.

### Answer

In our implementation, here's the exact sequence and final state:

```go
// Step 1: Withdraw from source
if err := sourceAccount.Withdraw(req.Amount); err != nil {
    return nil, err  // ← We'd return here if source fails
}

// If we reach here, source.Withdraw() succeeded:
// - sourceAccount.balance has been decreased
// - sourceAccount.Changes.MarkDirty("balance") was called

// Step 2: Deposit to destination
if err := destinationAccount.Deposit(req.Amount); err != nil {
    return nil, err  // ← If dest fails, we return HERE
}
// If we reach here, both succeeded
```

#### Scenario: Transfer $250 from Account A ($1000) to Account B ($500), but B is FROZEN

**Initial State:**
- Account A: balance=100000 (cents), status=ACTIVE
- Account B: balance=50000, status=FROZEN

**Execution:**

1. `source.Withdraw(25000)` → **SUCCEEDS**
   - Checks: amount positive ✓, account active ✓, sufficient funds ✓
   - Updates: `sourceAccount.balance = 75000`
   - Marks: `sourceAccount.Changes.MarkDirty("balance")`

2. `destination.Deposit(25000)` → **FAILS with ErrAccountInactive**
   - Checks: amount positive ✓, **account NOT active ✗**
   - Returns: `ErrAccountInactive` error

3. `return nil, ErrAccountInactive` → **EXIT EARLY**
   - No mutations collected
   - Plan is `nil`

**Final State:**

| Component | State |
|-----------|-------|
| In-Memory sourceAccount | balance=75000, dirty=true |
| In-Memory destAccount | balance=50000, dirty=false |
| Database | **UNCHANGED** (no mutations applied) |
| Returned Plan | `nil` |
| Returned Error | `ErrAccountInactive` |

### Why This Is Safe

The **in-memory objects are modified, but the database is untouched** because:

1. **No mutations were collected** - `plan.Add()` was never called
2. **Service layer receives `nil` plan** - So it applies nothing to database
3. **Application layer sees the error** - Can log it, notify user, etc.
4. **Next operation gets fresh account** - Next `Retrieve()` gets unmodified data

### The Real Issue

This is **only safe if the caller properly handles the error**. If we did:

```go
plan, err := uc.Execute(ctx, req)
if err != nil {
    // GOOD: Handle error, don't apply plan
    return err
}
// Apply plan
service.ApplyPlan(plan)
```

But if the caller forgot error checking:

```go
plan, _ := uc.Execute(ctx, req)  // ← Ignoring error!
service.ApplyPlan(plan)  // ← Applies nil, which is safe but wrong
// But now it seems like transfer succeeded when it failed!
```

### Comparison: Buggy Code's Behavior

The buggy code would produce **inconsistent database state**:

```go
source.balance -= req.Amount  // source.balance = 75000 (in memory)
dest.balance += req.Amount    // dest.balance = 75000 (in memory)

uc.db.Apply(mutation1)  // ← Applies! Source now 75000 in DB
uc.db.Apply(mutation2)  // ← FAILS! Destination still 50000 in DB

return err  // ← Too late! One mutation applied, one didn't
```

**Result**: Money disappeared from system (75000 left A, 50000 still in B, missing 25000)

---

## Q2: Why One-at-a-Time Mutation Application Is Problematic

**Question**: The buggy code applies mutations one at a time. Why is this a problem? Give a specific failure scenario.

### Answer

Applying mutations one-at-a-time violates **ACID transactional guarantees** and creates **inconsistency windows**.

### Specific Failure Scenario

**Setup**: Transfer $200 from Account A to Account B

**Buggy Code Flow:**
```
DB State:  A=$1000, B=$500
[Transfer requested]
mutation1 = {account_id: "A", balance: 800}
mutation2 = {account_id: "B", balance: 700}

Time 1: uc.db.Apply(mutation1) ← SUCCEEDS, writes to DB
  DB State:  A=$800, B=$500  ← INCONSISTENT!

Time 2: Network fails / concurrent delete / permission error
        uc.db.Apply(mutation2) ← FAILS!
  DB State:  A=$800, B=$500  ← STILL INCONSISTENT!

return error
```

### Resulting State

| Account | Expected | Actual | Problem |
|---------|----------|--------|---------|
| A | $800 | $800 | ✓ Correct |
| B | $700 | $500 | ✗ **Money not received** |
| System | Total=$1500 | Total=$1300 | ✗ **$200 LOST** |

### Why Databases See This Problem

From the database's perspective:
1. **Atomicity violation**: Some mutations of a transaction succeed, others fail
2. **Consistency violation**: Invariant broken (total money != before)
3. **Observability issue**: Another process might read after mutation1 but before mutation2 fails

```
Process X (Monitor):
  - Reads A=$800, B=$500 ← SEES THE INCONSISTENCY
  - Reports: "Money disappeared!"
```

### Real-World Scenario

**Scenario**: Bank system with audit logging

```
Time sequence:
1. Transfer begins: A=$1000, B=$500
2. Mutation1 applied: A=$800 (DB written)
3. [Audit process reads] ← Sees A=$800, B=$500
4. [Audit logs inconsistency] ← Creates ERROR alert
5. Mutation2 fails: B still $500
6. Error returned to client
7. Client: "Transfer failed"
8. Audit team: "WTF? Why is database inconsistent?"
9. Manual intervention required, data repair needed
```

### Why Atomicity Matters

**Atomic** means either:
- ALL mutations succeed, OR
- NO mutations succeed (all rolled back)

**Never**: Some succeed, some fail

Our correct approach:

```go
plan := &Plan{ mutation1, mutation2 }
service.ApplyAtomically(plan)  // ← ALL or NOTHING
```

The service layer can:
- Wrap both in a database transaction
- Commit only if both succeed
- Rollback if either fails
- Ensure consistency

---

## Q3: Why Dirty Field Tracking Matters for Concurrent Updates

**Question**: Your `UpdateMut` should only include dirty fields. If an account has balance changed but status unchanged, the mutation should NOT include status. Why does this matter for concurrent updates?

### Answer

**Without dirty field tracking**, concurrent updates suffer from the **Lost Update Anomaly**.

### Scenario: Concurrent Admin Action + User Transfer

**Initial State:**
- Account ID: "account-123"
- Balance: 1000 cents ($10.00)
- Status: "ACTIVE"

**Two concurrent operations:**
- **Transaction A** (Admin): Freeze account (status ACTIVE → FROZEN)
- **Transaction B** (User): Transfer $3 (balance 1000 → 970)

### Without Dirty Fields (BUGGY - Includes All Fields)

```
Time 1 (0ms):
  T-A reads: balance=1000, status=ACTIVE
  T-B reads: balance=1000, status=ACTIVE

Time 2 (10ms):
  T-A applies: {balance: 1000, status: "FROZEN"}  ← Includes ALL fields!
  DB now: balance=1000, status=FROZEN ✓

Time 3 (20ms):
  T-B applies: {balance: 970, status: "ACTIVE"}  ← Includes ALL fields!
  DB now: balance=970, status="ACTIVE" ✗ WRONG!

Result: Account unfroze itself! Status reverted to ACTIVE
        Admin action was LOST
```

**Invariant broken**: Frozen accounts should stay frozen

### With Dirty Fields (CORRECT - Only Changed Fields)

```
Time 1 (0ms):
  T-A reads: balance=1000, status=ACTIVE
  T-B reads: balance=1000, status=ACTIVE

Time 2 (10ms):
  T-A creates mutation: {status: "FROZEN"}  ← ONLY status changed
  T-A applies: "UPDATE accounts SET status='FROZEN' WHERE id='account-123'"
  DB now: balance=1000, status=FROZEN ✓

Time 3 (20ms):
  T-B creates mutation: {balance: 970}  ← ONLY balance changed
  T-B applies: "UPDATE accounts SET balance=970 WHERE id='account-123'"
  DB now: balance=970, status=FROZEN ✓ BOTH CORRECT!

Result: Both changes persisted correctly
        No lost update
```

**Invariant maintained**: Frozen account stays frozen, balance updated

### Implementation Detail

In our code:

```go
// Repository.UpdateMut() - Only includes dirty fields
func (r *AccountRepo) UpdateMut(account *Account) *Mutation {
    updates := make(map[string]interface{})

    if account.Changes.IsDirty("balance") {
        updates["balance"] = account.Balance()  // ← Only if dirty
    }

    if account.Changes.IsDirty("status") {
        updates["status"] = account.Status()  // ← Only if dirty
    }

    if len(updates) == 0 {
        return nil  // ← No changes
    }

    return &Mutation{
        Table:   "accounts",
        ID:      string(account.ID()),
        Updates: updates,
    }
}
```

### Why This Prevents Lost Updates

Because only **changed fields are in the mutation**, concurrent transactions don't interfere:

- T-A's `SET status=...` doesn't touch balance
- T-B's `SET balance=...` doesn't touch status
- Each transaction's SQL is independent

**With all fields**: Every update was a full replacement, causing conflicts

**With dirty fields**: Each update is surgical (only changed fields)

---

## Q4: Problems with Always Including All Fields

**Question**: Look at this alternative approach that always includes all fields. What problem does this cause that the dirty-field approach avoids?

### Alternative (Problematic)
```go
func (r *AccountRepo) UpdateMut(account *Account) *Mutation {
    return &Mutation{
        Updates: map[string]interface{}{
            "balance": account.Balance(),
            "status":  account.Status(),  // Always include all fields
        },
    }
}
```

### Answer

The always-include-all-fields approach causes **the Lost Update Anomaly** and is vulnerable to **write-write conflicts**.

### Problem 1: Lost Update Anomaly (Demonstrated Above)

When each transaction includes all fields, the last write wins regardless of what changed:

```
Account state: {id: 1, balance: 1000, status: ACTIVE, lastModified: T1}

Transaction A (0ms):  Admin freezes account
  - Reads: {balance: 1000, status: ACTIVE}
  - Modifies: status → FROZEN
  - Creates mutation: {balance: 1000, status: FROZEN}  ← ALL fields
  - Writes to DB (20ms): Account is FROZEN

Transaction B (5ms): User transfers $100
  - Reads: {balance: 1000, status: ACTIVE}  ← Read before A's write!
  - Modifies: balance → 900
  - Creates mutation: {balance: 900, status: ACTIVE}  ← ALL fields!!!
  - Writes to DB (25ms): Account is ACTIVE again (lost freeze!)
```

### Problem 2: Race Condition on Simple Updates

```go
// What if balance changes but we still include "status: ACTIVE"?

Scenario:
- DB: {id: 1, balance: 1000, status: FROZEN}
- Update: transfer reduces balance to 950
- Mutation created: {balance: 950, status: FROZEN}  ← Still correct by chance

But if code generates:
- Mutation created: {balance: 950, status: ACTIVE}  ← STALE "ACTIVE"!
- DB should be: {balance: 950, status: FROZEN}
- DB becomes: {balance: 950, status: ACTIVE}  ← LOST the freeze!
```

### Problem 3: Inefficient Updates

```go
// Unnecessary field updates
UPDATE accounts SET balance=1000, status='ACTIVE' WHERE id=1
```

vs (dirty field approach):

```go
// Only what changed
UPDATE accounts SET balance=950 WHERE id=1
```

The inefficiency causes:
- More database write volume
- Locks held longer
- Increased risk of conflicts
- Slower concurrent throughput

### Problem 4: Incorrect Semantics in ORM/SQL

Many ORMs treat "include all fields" as "replace entire object":

```go
// Scenario: Concurrent operations on same account

// With dirty fields: Only touch what changed
UPDATE accounts SET balance = ? WHERE id = ?  ← Safe, targeted

// With all fields: Unintended overwrites
UPDATE accounts SET balance = ?, status = ?, role = ?, lastModified = ? WHERE id = ?
// If another process set status AFTER we read but BEFORE we write,
// this statement OVERWRITES that change with the stale value
```

### Real-World Consequence

**Case Study**: E-commerce checkout

```
Inventory record: {product_id: 1, stock: 100, price: $10, status: ACTIVE}

User A (Shopper):
  - Item is in stock, creates order
  - Reads: stock=100, status=ACTIVE
  - [Reduces stock to 98]
  - Creates: {stock: 98, price: $10, status: ACTIVE}
  
Admin B:
  - Decides to discount: price $10 → $5
  - Reads: stock=100, price=$10, status=ACTIVE
  - Updates: {stock: 100, price: $5, status: ACTIVE}  ← Stock reset!

Result:
- Coffee was $10, now $5 ✓ Admin sees it worked
- But stock was 98, now shows 100 ✗ Overselling!
- Inventory mismatch
```

### Dirty Field Approach Avoids All These

```go
// User A writes: {stock: 98}
UPDATE accounts SET stock = 98 WHERE id = 1

// Admin B writes: {price: 5}
UPDATE accounts SET price = 5 WHERE id = 1

// Result: {stock: 98, price: 5, status: ACTIVE}  ← BOTH correct!
```

### Why Dirty Fields Are Critical

1. **Prevents Lost Updates**: Only changed fields in mutation
2. **No Stale Data Overwrites**: Doesn't revert other concurrent changes
3. **Efficiency**: Smaller updates, faster execution
4. **Correctness**: Preserves database invariants
5. **Observability**: Only modified fields appear in mutation record, clearer audit trail

This is why databases have optimistic locking patterns like:
- Version/revision numbers
- Last-modified timestamps
- Shadowed writes

And why ORMs provide dirty tracking as a feature, not a bug.
