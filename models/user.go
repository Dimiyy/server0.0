package models

import "time"

type User struct {
    ID           int       `json:"id"`
    Login        string    `json:"login"`
    PasswordHash string    `json:"-"` // Пароль не должен возвращаться в JSON
    Position     string    `json:"position"`
    Department   string    `json:"department"`
    Phone        string    `json:"phone,omitempty"`
    Email        string    `json:"email,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type UserRole struct {
    UserID       int    `json:"user_id"`
    Role         string `json:"role"`
    Department   string `json:"department,omitempty"` // Для ролей, привязанных к подразделению
}