package contracts

import "context"

// AccountID is a unique identifier for an account
type AccountID string

// AccountStatus represents the status of an account
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "ACTIVE"
	AccountStatusInactive AccountStatus = "INACTIVE"
	AccountStatusFrozen   AccountStatus = "FROZEN"
)

// ChangeTracker tracks changes to an entity for dirty-field detection
type ChangeTracker struct {
	dirtyFields map[string]bool
}

// NewChangeTracker creates a new change tracker
func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{
		dirtyFields: make(map[string]bool),
	}
}

// MarkDirty marks a field as changed
func (ct *ChangeTracker) MarkDirty(field string) {
	if ct.dirtyFields == nil {
		ct.dirtyFields = make(map[string]bool)
	}
	ct.dirtyFields[field] = true
}

// IsDirty checks if a field has been marked as changed
func (ct *ChangeTracker) IsDirty(field string) bool {
	return ct.dirtyFields[field]
}

// HasChanges returns true if any fields have been changed
func (ct *ChangeTracker) HasChanges() bool {
	return len(ct.dirtyFields) > 0
}

// Mutation represents a mutation to be applied to a record
type Mutation struct {
	Table   string
	ID      string
	Updates map[string]interface{}
}

// Plan represents a collection of mutations to be applied atomically
type Plan struct {
	mutations []*Mutation
}

// NewPlan creates a new plan
func NewPlan() *Plan {
	return &Plan{}
}

// Add adds a mutation to the plan
func (p *Plan) Add(m *Mutation) {
	if m != nil {
		p.mutations = append(p.mutations, m)
	}
}

// Mutations returns a copy of all mutations in the plan
func (p *Plan) Mutations() []*Mutation {
	result := make([]*Mutation, len(p.mutations))
	copy(result, p.mutations)
	return result
}

// IsEmpty returns true if the plan has no mutations
func (p *Plan) IsEmpty() bool {
	return len(p.mutations) == 0
}

// AccountRepository defines the interface for account persistence
type AccountRepository interface {
	// Retrieve fetches an account by ID
	Retrieve(ctx context.Context, id AccountID) (*Account, error)
	// UpdateMut returns a mutation for account changes (does NOT apply)
	UpdateMut(account *Account) *Mutation
}

// Account is imported from domain package but defined here as an interface placeholder
// The actual implementation is in the domain package
type Account interface {
	// ID returns the account ID
	ID() AccountID
	// Balance returns the current balance in cents
	Balance() int64
	// Status returns the account status
	Status() AccountStatus
	// Withdraw withdraws an amount from the account
	Withdraw(amount int64) error
	// Deposit deposits an amount to the account
	Deposit(amount int64) error
}
