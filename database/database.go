package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"supportTicketBotGo/config"
	"supportTicketBotGo/logger"

	_ "github.com/lib/pq"
)

var (
	DB    *sql.DB
	once  sync.Once
	stmts = make(map[string]*sql.Stmt)
	mutex sync.RWMutex
)

// Подготовленные запросы для максимальной производительности
var queries = map[string]string{
	"getUserByID":       "SELECT id, full_name, phone, location_lat, location_lng, birth_date, is_registered, registered_at FROM users WHERE id = $1",
	"createUser":        "INSERT INTO users (id) VALUES ($1) ON CONFLICT (id) DO NOTHING",
	"updateUser":        "UPDATE users SET full_name = $1, phone = $2, location_lat = $3, location_lng = $4, birth_date = $5, is_registered = $6, registered_at = $7 WHERE id = $8",
	"getActiveTickets":  "SELECT id, user_id, title, description, status, category, created_at, closed_at FROM tickets WHERE user_id = $1 AND status != 'закрыт' ORDER BY created_at DESC",
	"getTicketMessages": "SELECT id, ticket_id, sender_type, sender_id, message, created_at FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC",
	"addTicketMessage":  "INSERT INTO ticket_messages (ticket_id, sender_type, sender_id, message, created_at) VALUES ($1, $2, $3, $4, NOW()) RETURNING id",
	"createTicket":      "INSERT INTO tickets (user_id, title, description, status, category, created_at) VALUES ($1, $2, $3, 'open', $4, NOW()) RETURNING id",
	"closeTicket":       "UPDATE tickets SET status = 'закрыт', closed_at = NOW() WHERE id = $1 AND user_id = $2",
}

func ConnectDBOptimized() error {
	var err error
	once.Do(func() {
		dbConfig := config.AppConfig.Database
		connStr := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dbConfig.Host, dbConfig.Port, dbConfig.User,
			dbConfig.Password, dbConfig.DBName, dbConfig.SSLMode,
		)

		DB, err = sql.Open("postgres", connStr)
		if err != nil {
			return
		}

		// Оптимизированные настройки пула соединений
		DB.SetMaxOpenConns(50) // Увеличиваем количество соединений
		DB.SetMaxIdleConns(25) // Больше idle соединений
		DB.SetConnMaxLifetime(30 * time.Minute)
		DB.SetConnMaxIdleTime(5 * time.Minute)

		if err = DB.Ping(); err != nil {
			return
		}

		// Подготавливаем все запросы заранее
		for name, query := range queries {
			stmt, prepErr := DB.Prepare(query)
			if prepErr != nil {
				err = prepErr
				return
			}
			stmts[name] = stmt
		}

		logger.Info.Println("База данных оптимизирована и готова")
	})
	return err
}

func getStmt(name string) *sql.Stmt {
	mutex.RLock()
	defer mutex.RUnlock()
	return stmts[name]
}

// Оптимизированные функции с использованием подготовленных запросов
func GetUserByIDOptimized(userID int64) (*User, error) {
	stmt := getStmt("getUserByID")
	user := &User{}
	err := stmt.QueryRow(userID).Scan(
		&user.ID, &user.FullName, &user.Phone, &user.LocationLat,
		&user.LocationLng, &user.BirthDate, &user.IsRegistered, &user.RegisteredAt,
	)
	return user, err
}

func IsUserRegisteredOptimized(userID int64) (bool, error) {
	stmt := getStmt("isUserRegistered")
	var isRegistered bool
	err := stmt.QueryRow(userID).Scan(&isRegistered)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return isRegistered, err
}

func CreateUserOptimized(userID int64) error {
	stmt := getStmt("createUser")
	_, err := stmt.Exec(userID)
	return err
}

func CloseDB() {
	if DB != nil {
		// Закрываем все подготовленные запросы
		for _, stmt := range stmts {
			stmt.Close()
		}
		DB.Close()
	}
}

// Структуры для работы с базой данных

// User представляет пользователя системы
type User struct {
	ID           int64
	FullName     string
	Phone        string
	LocationLat  float64
	LocationLng  float64
	BirthDate    time.Time
	IsRegistered bool
	RegisteredAt time.Time
}

// Ticket представляет тикет поддержки
type Ticket struct {
	ID          int
	UserID      int64
	Title       string
	Description string
	Status      string
	Category    string
	CreatedAt   time.Time
	ClosedAt    sql.NullTime
}

// TicketMessage представляет сообщение в тикете
type TicketMessage struct {
	ID         int
	TicketID   int
	SenderType string
	SenderID   int64
	Message    string
	CreatedAt  time.Time
}

// TicketPhoto представляет фотографию, прикрепленную к тикету
type TicketPhoto struct {
	ID         int
	TicketID   int
	SenderType string
	SenderID   int64
	FilePath   string
	FileID     string
	MessageID  int
	CreatedAt  time.Time
}

// Функции для работы с пользователями

// CreateUser создает нового пользователя
func CreateUser(userID int64) error {
	_, err := DB.Exec(
		"INSERT INTO users (id) VALUES ($1) ON CONFLICT (id) DO NOTHING",
		userID,
	)
	return err
}

// UpdateUserRegistration обновляет данные регистрации пользователя
func UpdateUserRegistration(user *User) error {
	_, err := DB.Exec(
		`UPDATE users SET 
		full_name = $1, 
		phone = $2, 
		location_lat = $3, 
		location_lng = $4, 
		birth_date = $5, 
		is_registered = $6, 
		registered_at = $7 
		WHERE id = $8`,
		user.FullName, user.Phone, user.LocationLat, user.LocationLng,
		user.BirthDate, user.IsRegistered, time.Now(), user.ID,
	)
	return err
}

// CloseTicket закрывает тикет пользователя
func CloseTicket(ticketID int, userID int64) error {
	stmt := getStmt("closeTicket")
	result, err := stmt.Exec(ticketID, userID)
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}

	// Проверяем, был ли тикет действительно обновлен
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка при получении количества обновленных строк: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("тикет #%d не найден или не принадлежит пользователю %d", ticketID, userID)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("тикет не найден или не принадлежит пользователю")
	}

	return nil
}

// GetUserByID получает пользователя по ID
func GetUserByID(userID int64) (*User, error) {
	user := &User{}
	err := DB.QueryRow(
		`SELECT id, full_name, phone, location_lat, location_lng, 
		birth_date, is_registered, registered_at FROM users WHERE id = $1`,
		userID,
	).Scan(
		&user.ID, &user.FullName, &user.Phone, &user.LocationLat,
		&user.LocationLng, &user.BirthDate, &user.IsRegistered, &user.RegisteredAt,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// IsUserRegistered проверяет, зарегистрирован ли пользователь
func IsUserRegistered(userID int64) (bool, error) {
	var isRegistered bool
	err := DB.QueryRow("SELECT is_registered FROM users WHERE id = $1", userID).Scan(&isRegistered)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return isRegistered, nil
}

// Функции для работы с тикетами

// CreateTicket создает новый тикет
func CreateTicket(ticket *Ticket) (int, error) {
	var ticketID int
	err := DB.QueryRow(
		`INSERT INTO tickets (user_id, title, description, status, category, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		ticket.UserID, ticket.Title, ticket.Description, ticket.Status,
		ticket.Category, time.Now(),
	).Scan(&ticketID)
	return ticketID, err
}

// GetActiveTicketsByUserID получает активные тикеты пользователя
func GetActiveTicketsByUserID(userID int64) ([]Ticket, error) {
	rows, err := DB.Query(
		`SELECT id, user_id, title, description, status, category, created_at, closed_at 
		FROM tickets WHERE user_id = $1 AND status != 'закрыт' ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status,
			&t.Category, &t.CreatedAt, &t.ClosedAt,
		); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}

	return tickets, nil
}

// GetTicketHistory получает историю тикетов пользователя
func GetTicketHistory(userID int64) ([]Ticket, error) {
	rows, err := DB.Query(
		`SELECT id, user_id, title, description, status, category, created_at, closed_at 
		FROM tickets WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status,
			&t.Category, &t.CreatedAt, &t.ClosedAt,
		); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}

	return tickets, nil
}

// Функции для работы с сообщениями тикетов

// AddTicketMessage добавляет новое сообщение в тикет и возвращает его ID
func AddTicketMessage(message *TicketMessage) (int, error) {
	var messageID int
	err := DB.QueryRow(
		`INSERT INTO ticket_messages 
		(ticket_id, sender_type, sender_id, message, created_at) 
		VALUES ($1, $2, $3, $4, NOW()) RETURNING id`,
		message.TicketID, message.SenderType, message.SenderID, message.Message,
	).Scan(&messageID)

	message.ID = messageID // Устанавливаем ID в структуре
	return messageID, err
}

// GetTicketMessages получает все сообщения тикета
func GetTicketMessages(ticketID int) ([]TicketMessage, error) {
	rows, err := DB.Query(
		`SELECT id, ticket_id, sender_type, sender_id, message, created_at 
		FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []TicketMessage
	for rows.Next() {
		var m TicketMessage
		if err := rows.Scan(
			&m.ID, &m.TicketID, &m.SenderType, &m.SenderID,
			&m.Message, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// GetTicketMessageCount возвращает количество сообщений в тикете
func GetTicketMessageCount(ticketID int) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM ticket_messages WHERE ticket_id = $1`,
		ticketID,
	).Scan(&count)
	return count, err
}

// GetTicketByID получает тикет по его ID
func GetTicketByID(ticketID int) (*Ticket, error) {
	ticket := &Ticket{}
	err := DB.QueryRow(
		`SELECT id, user_id, title, description, status, category, created_at, closed_at 
		FROM tickets WHERE id = $1`,
		ticketID,
	).Scan(
		&ticket.ID, &ticket.UserID, &ticket.Title, &ticket.Description,
		&ticket.Status, &ticket.Category, &ticket.CreatedAt, &ticket.ClosedAt,
	)
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

// UpdateTicketStatus обновляет статус тикета
func UpdateTicketStatus(ticketID int, status string) error {
	var err error
	if status == "закрыт" {
		_, err = DB.Exec(
			`UPDATE tickets SET status = $1, closed_at = NOW() WHERE id = $2`,
			status, ticketID,
		)
	} else {
		_, err = DB.Exec(
			`UPDATE tickets SET status = $1 WHERE id = $2`,
			status, ticketID,
		)
	}
	return err
}

// GetUserNameByID возвращает имя пользователя по ID
func GetUserNameByID(userID int64) (string, error) {
	var fullName string
	err := DB.QueryRow(
		`SELECT full_name FROM users WHERE id = $1`,
		userID,
	).Scan(&fullName)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("пользователь не найден")
		}
		return "", err
	}

	if fullName == "" {
		return "Сотрудник поддержки", nil
	}

	return fullName, nil
}

// AddTicketPhoto добавляет информацию о фотографии в базу данных
func AddTicketPhoto(photo *TicketPhoto) (int, error) {
	var photoID int
	err := DB.QueryRow(
		`INSERT INTO ticket_photos (ticket_id, sender_type, sender_id, file_path, file_id, message_id, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		photo.TicketID, photo.SenderType, photo.SenderID, photo.FilePath, photo.FileID, photo.MessageID, time.Now(),
	).Scan(&photoID)

	return photoID, err
}

// GetTicketPhotos получает все фотографии для заданного тикета
func GetTicketPhotos(ticketID int) ([]TicketPhoto, error) {
	rows, err := DB.Query(
		`SELECT id, ticket_id, sender_type, sender_id, file_path, file_id, message_id, created_at 
		FROM ticket_photos 
		WHERE ticket_id = $1 
		ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []TicketPhoto
	for rows.Next() {
		var p TicketPhoto
		if err := rows.Scan(
			&p.ID, &p.TicketID, &p.SenderType, &p.SenderID,
			&p.FilePath, &p.FileID, &p.MessageID, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}

	return photos, nil
}
