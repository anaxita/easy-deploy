package entity

import (
	"github.com/gofrs/uuid/v5"
)

type Plan int8

const (
	PlanFree Plan = iota
	PlanPremium
)

type User struct {
	ID       uuid.UUID `json:"id"`       // Генерируем автоматически
	Email    string    `json:"email"`    // Уникальный
	Password string    `json:"password"` // Хешируем
	Plan     Plan      `json:"plan"`     // По умолчанию PlanFree
}
0
