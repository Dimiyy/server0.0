package admin

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"

	"net/http"
	"strings"
	"html/template"

	"server/models"
)

var (
	ErrUserExists      = errors.New("пользователь с таким логином уже существует")
	ErrInvalidRole     = errors.New("неверная роль пользователя")
	ErrUserNotFound    = errors.New("пользователь не найден")
	ErrEmptyLogin      = errors.New("логин не может быть пустым")
	ErrEmptyPassword   = errors.New("пароль не может быть пустым")
	ErrEmptyPosition   = errors.New("должность не может быть пустой")
	ErrEmptyDepartment = errors.New("подразделение не может быть пустым")
)

type AdminHandler struct {
	db *sql.DB
}

func NewAdminHandler(db *sql.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// RegisterHandlers регистрирует обработчики для административных функций
func (h *AdminHandler) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/admin/register", h.registerUserHandler)
	mux.HandleFunc("/admin/assign-roles", h.assignRolesHandler)
	mux.HandleFunc("/admin/users", h.listUsersHandler)
}

// hashPassword создает хеш пароля
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// registerUserHandler обрабатывает регистрацию нового пользователя
func (h *AdminHandler) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showRegistrationForm(w, r)
	case http.MethodPost:
		h.processRegistrationForm(w, r)
	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

// showRegistrationForm отображает форму регистрации
func (h *AdminHandler) showRegistrationForm(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/admin/register.html")
	if err != nil {
		http.Error(w, "Ошибка загрузки шаблона: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Ошибка рендеринга шаблона: "+err.Error(), http.StatusInternalServerError)
	}
}

// processRegistrationForm обрабатывает данные формы регистрации
func (h *AdminHandler) processRegistrationForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ошибка обработки формы: "+err.Error(), http.StatusBadRequest)
		return
	}

	user := models.User{
		Login:      strings.TrimSpace(r.FormValue("login")),
		Position:   strings.TrimSpace(r.FormValue("position")),
		Department: strings.TrimSpace(r.FormValue("department")),
		Phone:      strings.TrimSpace(r.FormValue("phone")),
		Email:      strings.TrimSpace(r.FormValue("email")),
	}
	password := r.FormValue("password")

	// Валидация
	if user.Login == "" {
		http.Error(w, ErrEmptyLogin.Error(), http.StatusBadRequest)
		return
	}
	if password == "" {
		http.Error(w, ErrEmptyPassword.Error(), http.StatusBadRequest)
		return
	}
	if user.Position == "" {
		http.Error(w, ErrEmptyPosition.Error(), http.StatusBadRequest)
		return
	}
	if user.Department == "" {
		http.Error(w, ErrEmptyDepartment.Error(), http.StatusBadRequest)
		return
	}

	// Хеширование пароля
	user.PasswordHash = hashPassword(password)

	// Сохранение в БД
	if err := h.createUser(&user); err != nil {
		http.Error(w, "Ошибка создания пользователя: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users?success=1", http.StatusSeeOther)
}

// createUser сохраняет пользователя в БД
func (h *AdminHandler) createUser(user *models.User) error {
	// Проверка на существование пользователя
	var exists bool
	err := h.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE login = ?)", user.Login).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserExists
	}

	// Вставка нового пользователя
	_, err = h.db.Exec(`
		INSERT INTO users (login, password_hash, position, department, phone, email)
		VALUES (?, ?, ?, ?, ?, ?)`,
		user.Login, user.PasswordHash, user.Position, user.Department, user.Phone, user.Email)

	return err
}

// assignRolesHandler обрабатывает назначение ролей пользователю
func (h *AdminHandler) assignRolesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showAssignRolesForm(w, r)
	case http.MethodPost:
		h.processAssignRolesForm(w, r)
	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

// showAssignRolesForm отображает форму назначения ролей
func (h *AdminHandler) showAssignRolesForm(w http.ResponseWriter, r *http.Request) {
	// Получаем список пользователей для выпадающего списка
	users, err := h.getUsersList()
	if err != nil {
		http.Error(w, "Ошибка получения списка пользователей: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Users []models.User
	}{
		Users: users,
	}

	tmpl, err := template.ParseFiles("templates/admin/assign_roles.html")
	if err != nil {
		http.Error(w, "Ошибка загрузки шаблона: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Ошибка рендеринга шаблона: "+err.Error(), http.StatusInternalServerError)
	}
}

// processAssignRolesForm обрабатывает данные формы назначения ролей
func (h *AdminHandler) processAssignRolesForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ошибка обработки формы: "+err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	if userID == "" {
		http.Error(w, "Не выбран пользователь", http.StatusBadRequest)
		return
	}

	// Доступные роли
	availableRoles := []string{
		"Руководитель ИТЦ",
		"ГПТО ИТЦ",
		"Руководитель СП ИТЦ",
		"Специалист СП ИТЦ",
		"УТТСТ",
		"ОК ИТЦ",
	}

	// Начинаем транзакцию
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "Ошибка начала транзакции: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Удаляем все текущие роли пользователя
	_, err = tx.Exec("DELETE FROM user_roles WHERE user_id = ?", userID)
	if err != nil {
		http.Error(w, "Ошибка удаления текущих ролей: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Добавляем новые роли
	for _, role := range availableRoles {
		if r.FormValue(role) == "on" {
			_, err = tx.Exec(`
				INSERT INTO user_roles (user_id, role, department)
				VALUES (?, ?, ?)`,
				userID, role, r.FormValue(role+"_department"))
			if err != nil {
				http.Error(w, "Ошибка назначения роли: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Коммитим транзакцию
	if err = tx.Commit(); err != nil {
		http.Error(w, "Ошибка сохранения изменений: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users?success=1", http.StatusSeeOther)
}

// listUsersHandler отображает список пользователей
func (h *AdminHandler) listUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.getUsersWithRoles()
	if err != nil {
		http.Error(w, "Ошибка получения списка пользователей: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Users []UserWithRoles
	}{
		Users: users,
	}

	tmpl, err := template.ParseFiles("templates/admin/users_list.html")
	if err != nil {
		http.Error(w, "Ошибка загрузки шаблона: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Ошибка рендеринга шаблона: "+err.Error(), http.StatusInternalServerError)
	}
}

// UserWithRoles объединяет данные пользователя и его роли
type UserWithRoles struct {
	models.User
	Roles []models.UserRole
}

// getUsersWithRoles возвращает список пользователей с их ролями
func (h *AdminHandler) getUsersWithRoles() ([]UserWithRoles, error) {
	// Получаем всех пользователей
	users, err := h.getUsersList()
	if err != nil {
		return nil, err
	}

	// Получаем роли для каждого пользователя
	var result []UserWithRoles
	for _, user := range users {
		roles, err := h.getUserRoles(user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, UserWithRoles{
			User:  user,
			Roles: roles,
		})
	}

	return result, nil
}

// getUsersList возвращает список всех пользователей
func (h *AdminHandler) getUsersList() ([]models.User, error) {
	rows, err := h.db.Query("SELECT id, login, position, department, phone, email FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Login, &user.Position, &user.Department, &user.Phone, &user.Email); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// getUserRoles возвращает роли пользователя
func (h *AdminHandler) getUserRoles(userID int) ([]models.UserRole, error) {
	rows, err := h.db.Query("SELECT role, department FROM user_roles WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.UserRole
	for rows.Next() {
		var role models.UserRole
		if err := rows.Scan(&role.Role, &role.Department); err != nil {
			return nil, err
		}
		role.UserID = userID
		roles = append(roles, role)
	}

	return roles, nil
}