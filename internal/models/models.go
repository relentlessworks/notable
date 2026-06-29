package models

import "time"

// Note represents a note in the knowledge base.
type Note struct {
	Handle    string    `json:"handle"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Workspace represents a tenant in the system.
type Workspace struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Plan   string `json:"plan"`
}

// Token represents an auth token.
type Token struct {
	Value     string    `json:"value"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
