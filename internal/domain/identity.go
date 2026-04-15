package domain

import "time"

// User represents an authenticated account principal.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	FullName     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Organization is the tenant root for multi-tenant data isolation.
type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Membership links a user to an organization with a role.
type Membership struct {
	ID             string
	UserID         string
	OrganizationID string
	Role           string
	Status         string
	InvitedAt      *time.Time
	JoinedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
