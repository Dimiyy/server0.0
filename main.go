package main

import (
	"server/admin"

	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Добавьте новую структуру для заявки
type Application struct {
	AppNumber     string
	Department    string
	TransportType string
	StartDate     string
	EndDate       string
	WorkDate      string
	Track         string
	Charecter     string
	Workers       string
	Comment       string
	CreatedAt     time.Time
}

type dbApplication struct {
	AppNumber     string
	TransportType string
	Department    string
	StartDate     string
	EndDate       string
	WorkDate      string
	Track         string
	Charecter     string
	Workers       string
	Comment       string
	CreatedAt     time.Time
}

type Order struct {
	AppNumber      string
	DepartmentName string
	Dates          []string
}

// Добавлен отсутствующий тип applicationDetail
type applicationDetail struct {
	Date      string
	Track     string
	Charecter string
	Workers   string
	Comment   string
}

type viewModel struct {
	AppNumber     string
	TransportType string
	Department    string
	Period        string
	Details       []applicationDetail
}

func main() {
	db, err := sql.Open("mysql", "root:dimi@tcp(localhost:3306)/itc?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	// Применяем миграции
	if err := applyMigrations(db); err != nil {
		log.Fatal("Ошибка миграций: ", err)
	}

	// Инициализация административного модуля
	adminHandler := admin.NewAdminHandler(db)

	mux := http.NewServeMux()

	// Регистрируем обработчики администратора
	adminHandler.RegisterHandlers(mux)

	// Обработчики для статических файлов
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Основные обработчики
	http.HandleFunc("/", dashboardHandler(db))
	http.HandleFunc("/save_order", saveOrderHandler(db))
	http.HandleFunc("/applications", applicationsHandler(db))
	http.HandleFunc("/delete-applications", deleteApplicationsHandler(db))
	http.HandleFunc("/transport-request", transportRequestHandler(db)) // страница с формой заявки

	fmt.Println("Сервер запущен на :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func applyMigrations(db *sql.DB) error {
	// Проверяем, существует ли таблица users (простейшая проверка)
	var tableExists int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'").Scan(&tableExists)
	if err != nil {
		return err
	}

	if tableExists == 0 {
		// Применяем миграции
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			login VARCHAR(50) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			position VARCHAR(100) NOT NULL,
			department VARCHAR(100) NOT NULL,
			phone VARCHAR(20),
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_roles (
			user_id INT NOT NULL,
			role VARCHAR(50) NOT NULL,
			department VARCHAR(100),
			PRIMARY KEY (user_id, role),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`)
		if err != nil {
			return err
		}
	}

	return nil
}

// Обработчик для страницы списка заявок
func applicationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic in applicationsHandler: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		rows, err := db.Query(`
            SELECT 
                application_number,
                transport_type,
                department_name,
                DATE_FORMAT(start_date, '%d.%m.%Y') as start_date,
                DATE_FORMAT(end_date, '%d.%m.%Y') as end_date,
                DATE_FORMAT(work_date, '%d.%m.%Y') as work_date,
                track,
                charecter,
                workers,
                comment,
                created_at
            FROM itc
            ORDER BY created_at DESC
        `)
		if err != nil {
			http.Error(w, "Ошибка при получении заявок: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := rows.Close(); err != nil {
				log.Printf("Error closing rows: %v", err)
			}
		}()

		var apps []dbApplication
		for rows.Next() {
			var app dbApplication
			err := rows.Scan(
				&app.AppNumber,
				&app.TransportType,
				&app.Department,
				&app.StartDate,
				&app.EndDate,
				&app.WorkDate,
				&app.Track,
				&app.Charecter,
				&app.Workers,
				&app.Comment,
				&app.CreatedAt,
			)
			if err != nil {
				http.Error(w, "Ошибка при сканировании заявки: "+err.Error(), http.StatusInternalServerError)
				return
			}
			apps = append(apps, app)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Ошибка при итерации по заявкам: "+err.Error(), http.StatusInternalServerError)
			return
		}

		groupedApps := make(map[string][]dbApplication)
		for _, app := range apps {
			groupedApps[app.AppNumber] = append(groupedApps[app.AppNumber], app)
		}

		var viewModels []viewModel
		for appNumber, apps := range groupedApps {
			if len(apps) == 0 {
				continue
			}

			startDate := apps[0].StartDate
			endDate := apps[0].EndDate
			for _, app := range apps {
				if app.StartDate < startDate {
					startDate = app.StartDate
				}
				if app.EndDate > endDate {
					endDate = app.EndDate
				}
			}

			period := startDate
			if startDate != endDate {
				period = startDate + " - " + endDate
			}

			var details []applicationDetail
			for _, app := range apps {
				details = append(details, applicationDetail{
					Date:      app.WorkDate,
					Track:     app.Track,
					Charecter: app.Charecter,
					Workers:   app.Workers,
					Comment:   app.Comment,
				})
			}

			sort.Slice(details, func(i, j int) bool {
				return parseDate(details[i].Date).Before(parseDate(details[j].Date))
			})

			viewModels = append(viewModels, viewModel{
				AppNumber:     appNumber,
				TransportType: apps[0].TransportType,
				Department:    apps[0].Department,
				Period:        period,
				Details:       details,
			})
		}

		sort.Slice(viewModels, func(i, j int) bool {
			return findLatestDate(groupedApps[viewModels[i].AppNumber]).After(
				findLatestDate(groupedApps[viewModels[j].AppNumber]))
		})

		funcMap := template.FuncMap{
			"add": func(a, b int) int { return a + b },
			"sub": func(a, b int) int { return a - b },
			"len": func(s []viewModel) int { return len(s) },
		}

		tmpl := template.New("applications.html").Funcs(funcMap)
		tmpl, err = tmpl.ParseFiles("templates/applications.html")
		if err != nil {
			http.Error(w, "Ошибка загрузки шаблона: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, viewModels); err != nil {
			http.Error(w, "Ошибка рендеринга шаблона: "+err.Error(), http.StatusInternalServerError)
		}

	}

}

func parseDate(dateStr string) time.Time {
	parts := strings.Split(dateStr, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	day, err1 := strconv.Atoi(parts[0])
	month, err2 := strconv.Atoi(parts[1])
	year, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func findLatestDate(apps []dbApplication) time.Time {
	latest := time.Time{}
	for _, app := range apps {
		if app.CreatedAt.After(latest) {
			latest = app.CreatedAt
		}
	}
	return latest
}

// Обработчик для dashboard с вкладками
func dashboardHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("templates/dashboard.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Можно добавить данные для dashboard, например, статистику
		data := struct {
			Title string
		}{
			Title: "Панель управления ИТЦ",
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// Обработчик для страницы с формой заявки (вкладка "Специалист СП ИТЦ")
func transportRequestHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("templates/all.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			AppNumber string
		}{
			AppNumber: r.URL.Query().Get("success"),
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func indexHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("templates/all.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			AppNumber string
		}{
			AppNumber: r.URL.Query().Get("success"),
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func saveOrderHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		departmentName := r.FormValue("department_name")
		transportType := r.FormValue("transportType")
		startDate := r.FormValue("startDate")
		endDate := r.FormValue("endDate")
		dates := r.Form["dates[]"]
		tracks := r.Form["tracks[]"]
		charecters := r.Form["charecters[]"]
		workers := r.Form["workers[]"]
		comments := r.Form["comments[]"]

		if len(dates) != len(tracks) || len(dates) != len(charecters) || len(dates) != len(workers) || len(dates) != len(comments) {
			http.Error(w, "Несоответствие количества полей", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		appNumber, err := generateApplicationNumber(tx, departmentName)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for i := 0; i < len(dates); i++ {
			mysqlDate, err := convertToMySQLDate(dates[i])
			if err != nil {
				log.Printf("Ошибка преобразования даты: %v", err)
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			_, err = tx.Exec(
				`INSERT INTO itc 
					(application_number, department_name, transport_type, start_date, end_date, work_date, track, charecter, workers, comment) 
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				appNumber,
				departmentName,
				transportType,
				startDate,
				endDate,
				mysqlDate,
				tracks[i],
				charecters[i],
				workers[i],
				comments[i],
			)

			if err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/applications", http.StatusSeeOther)
	}
}

func generateApplicationNumber(tx *sql.Tx, departmentName string) (string, error) {
	year := time.Now().Year()

	var lastNumber int
	err := tx.QueryRow(
		"SELECT last_number FROM application_counters WHERE department_prefix = ? FOR UPDATE",
		departmentName,
	).Scan(&lastNumber)

	if err != nil {
		if err == sql.ErrNoRows {
			// Если запись не найдена, создаем новую
			lastNumber = 1
			_, err = tx.Exec(
				"INSERT INTO application_counters (department_prefix, last_number) VALUES (?, ?)",
				departmentName, lastNumber,
			)
			if err != nil {
				return "", fmt.Errorf("ошибка создания счетчика: %v", err)
			}
		} else {
			return "", fmt.Errorf("ошибка получения счетчика: %v", err)
		}
	} else {
		lastNumber++
	}

	_, err = tx.Exec(
		"UPDATE application_counters SET last_number = ? WHERE department_prefix = ?",
		lastNumber, departmentName,
	)

	if err != nil {
		return "", fmt.Errorf("ошибка обновления счетчика: %v", err)
	}

	return fmt.Sprintf("%s-%d-%04d", departmentName, year, lastNumber), nil
}

func convertToMySQLDate(date string) (string, error) {
	parts := strings.Split(date, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("неверный формат даты: %s", date)
	}

	// Проверяем, что все части даты являются числами
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return "", fmt.Errorf("неверный день: %s", parts[0])
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return "", fmt.Errorf("неверный месяц: %s", parts[1])
	}
	if _, err := strconv.Atoi(parts[2]); err != nil {
		return "", fmt.Errorf("неверный год: %s", parts[2])
	}

	return fmt.Sprintf("%s-%s-%s", parts[2], parts[1], parts[0]), nil
}

func deleteApplicationsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			Applications []string `json:"applications"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusBadRequest)
			return
		}

		if len(request.Applications) == 0 {
			http.Error(w, "Нет заявок для удаления", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Удаляем все выбранные заявки
		for _, appNumber := range request.Applications {
			_, err = tx.Exec("DELETE FROM itc WHERE application_number = ?", appNumber)
			if err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}
