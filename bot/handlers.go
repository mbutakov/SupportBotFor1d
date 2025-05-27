package bot

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"supportTicketBotGo/database"
	"supportTicketBotGo/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// UserState хранит состояние пользователя в боте
type UserState struct {
	State       string
	FullName    string
	Phone       string
	LocationLat float64
	LocationLng float64
	BirthDate   time.Time
	TicketTitle string
	TicketDesc  string
	TicketCat   string
	TicketID    int
}

// userStates хранит состояния всех пользователей
var userStates = make(map[int64]*UserState)

// --- СТАТУСЫ ТИКЕТОВ ---
const (
	StatusCreated        = "created"         // 🆕 Создан
	StatusAssigned       = "assigned"        // 👨‍💻 Назначен
	StatusInProgress     = "in_progress"     // 🔧 В работе
	StatusWaitingUser    = "waiting_user"    // ❓ Ожидает ответа пользователя
	StatusWaitingSupport = "waiting_support" // ⏳ Ожидает действий поддержки
	StatusResolved       = "resolved"        // ✅ Решён
	StatusClosed         = "closed"          // 🗃 Закрыт
	StatusCancelled      = "cancelled"       // 🚫 Отменён
)

// GetStatusEmojiAndText возвращает эмодзи и описание статуса тикета
func GetStatusEmojiAndText(status string) (string, string) {
	switch status {
	case StatusCreated:
		return "🆕", "Создан: тикет ожидает назначения агенту"
	case StatusAssigned:
		return "👨‍💻", "Назначен: ожидает начала работы агентом"
	case StatusInProgress:
		return "🔧", "В работе: агент работает над тикетом"
	case StatusWaitingUser:
		return "❓", "Ожидает ответа пользователя"
	case StatusWaitingSupport:
		return "⏳", "Ожидает действий поддержки"
	case StatusResolved:
		return "✅", "Решён: предложено решение, ожидает подтверждения"
	case StatusClosed:
		return "🗃", "Закрыт"
	case StatusCancelled:
		return "🚫", "Отменён: тикет не требует решения"
	default:
		return "❔", status
	}
}

// Валидация ФИО
func validateFullName(name string) bool {
	// Проверяем длину (минимум 2 слова, каждое не короче 2 символов)
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if len(part) < 2 {
			return false
		}
		// Проверяем, что все символы - буквы или допустимые знаки

	}

	return true
}

// Валидация даты рождения
func validateBirthDate(dateStr string) (time.Time, error) {
	// Пытаемся распарсить дату в формате DD.MM.YYYY
	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return time.Time{}, err
	}

	// Проверяем, что дата не в будущем
	if date.After(time.Now()) {
		return time.Time{}, fmt.Errorf("дата рождения не может быть в будущем")
	}

	// Проверяем, что возраст не меньше 14 лет
	minAge := time.Now().AddDate(-14, 0, 0)
	if date.After(minAge) {
		return time.Time{}, fmt.Errorf("возраст должен быть не менее 14 лет")
	}

	// Проверяем, что возраст не более 120 лет
	maxAge := time.Now().AddDate(-120, 0, 0)
	if date.Before(maxAge) {
		return time.Time{}, fmt.Errorf("возраст не может превышать 120 лет")
	}

	return date, nil
}

// Добавим новую функцию для создания директории uploads
func ensureUploadsDir() string {
	// Создаем путь к директории uploads на уровень выше текущей директории
	uploadsDir := "../uploads"

	// Проверяем существование директории
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		// Создаем директорию, если она не существует
		err := os.MkdirAll(uploadsDir, 0755)
		if err != nil {
			logger.Error.Printf("Ошибка при создании директории uploads: %v", err)
			// В случае ошибки используем текущую директорию
			return "uploads"
		}
	}

	return uploadsDir
}

// SendErrorMessage отправляет сообщение об ошибке
func SendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	SafeSendMessage(bot, msg)
}

// Создадим вспомогательную функцию для безопасной отправки сообщений
func SafeSendMessage(bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) {
	_, err := bot.Send(msg)
	if err != nil {
		logger.Error.Printf("Ошибка при отправке сообщения: %v", err)
	}
	// Удаляем задержку
}

// Функция для безопасной отправки фотографий
func SafeSendPhoto(bot *tgbotapi.BotAPI, photo tgbotapi.PhotoConfig) {
	_, err := bot.Send(photo)
	if err != nil {
		logger.Error.Printf("Ошибка при отправке фотографии: %v", err)
	}
	// Удаляем задержку
}

// Функция для безопасной отправки любого Chattable (сообщения, фото и т.д.)
func safeSend(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) {
	_, err := bot.Send(chattable)
	if err != nil {
		logger.Error.Printf("Ошибка при отправке: %v", err)
	}
	// Удаляем все задержки
}

// Добавляем новую функцию для сохранения аватара
func saveUserAvatar(bot *tgbotapi.BotAPI, userID int64) error {
	// Получаем фотографии профиля пользователя
	photos, err := bot.GetUserProfilePhotos(tgbotapi.UserProfilePhotosConfig{
		UserID: userID,
		Limit:  1,
	})
	if err != nil {
		return fmt.Errorf("ошибка при получении фото профиля: %v", err)
	}

	// Если у пользователя нет фотографий профиля, обновляем статус и выходим
	if photos.TotalCount == 0 {
		err = database.UpdateUserAvatar(userID, false)
		if err != nil {
			logger.Error.Printf("Ошибка при обновлении статуса аватара пользователя %d: %v", userID, err)
		}
		return nil
	}

	// Получаем самую последнюю фотографию в максимальном размере
	photo := photos.Photos[0][len(photos.Photos[0])-1]

	// Получаем файл
	fileURL, err := bot.GetFileDirectURL(photo.FileID)
	if err != nil {
		return fmt.Errorf("ошибка при получении URL фото: %v", err)
	}

	// Создаем директорию для аватаров, если её нет
	avatarsDir := filepath.Join("..", "uploads", "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		return fmt.Errorf("ошибка при создании директории аватаров: %v", err)
	}

	// Скачиваем файл
	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("ошибка при скачивании фото: %v", err)
	}
	defer resp.Body.Close()

	// Создаем файл для сохранения
	avatarPath := filepath.Join(avatarsDir, fmt.Sprintf("%d.png", userID))
	file, err := os.Create(avatarPath)
	if err != nil {
		return fmt.Errorf("ошибка при создании файла: %v", err)
	}
	defer file.Close()

	// Копируем содержимое
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении фото: %v", err)
	}

	// Обновляем статус аватара в базе данных
	err = database.UpdateUserAvatar(userID, true)
	if err != nil {
		logger.Error.Printf("Ошибка при обновлении статуса аватара пользователя %d: %v", userID, err)
	}

	return nil
}

// Обработчик команды /start
func HandleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

	// Создаем запись пользователя в БД, если ее еще нет
	err := database.CreateUser(userID)
	if err != nil {
		logger.Error.Printf("Ошибка при создании пользователя %d: %v", userID, err)
		SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при регистрации")
		return
	}

	// Проверяем, зарегистрирован ли пользователь
	isRegistered, err := database.IsUserRegistered(userID)
	if err != nil {
		logger.Error.Printf("Ошибка при проверке регистрации %d: %v", userID, err)
		SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при проверке регистрации")
		return
	}

	if isRegistered {
		// Если пользователь уже зарегистрирован, показываем главное меню
		msg := tgbotapi.NewMessage(message.Chat.ID, "Добро пожаловать в систему поддержки!")
		msg.ReplyMarkup = GetMainMenuKeyboard()
		SafeSendMessage(bot, msg)
	} else {
		// Начинаем процесс регистрации
		userStates[userID] = &UserState{State: "awaiting_fullname"}

		msg := tgbotapi.NewMessage(message.Chat.ID,
			"Добро пожаловать в систему поддержки! Для начала работы необходимо зарегистрироваться.\n\n"+
				"Пожалуйста, введите ваше полное имя (Фамилия Имя Отчество):")
		SafeSendMessage(bot, msg)
	}
}

// Обработчик сообщений в зависимости от состояния пользователя
func HandleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID
	state, exists := userStates[userID]

	// Если состояние не существует, создаем новое и начинаем регистрацию
	if !exists {
		// Проверяем, зарегистрирован ли пользователь
		isRegistered, err := database.IsUserRegistered(userID)
		if err != nil {
			logger.Error.Printf("Ошибка при проверке регистрации %d: %v", userID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при проверке регистрации")
			return
		}

		if isRegistered {
			// Обрабатываем сообщение как команду в главном меню
			HandleMainMenu(bot, message)
			return
		} else {
			// Начинаем процесс регистрации
			err := database.CreateUser(userID)
			if err != nil {
				logger.Error.Printf("Ошибка при создании пользователя %d: %v", userID, err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при регистрации")
				return
			}
			userStates[userID] = &UserState{State: "awaiting_fullname"}

			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Для начала работы необходимо зарегистрироваться.\n\n"+
					"Пожалуйста, введите ваше полное имя (Фамилия Имя Отчество):")
			SafeSendMessage(bot, msg)
			return
		}
	}

	// Обрабатываем сообщение в соответствии с текущим состоянием пользователя
	switch state.State {
	case "awaiting_fullname":
		// Проверяем ФИО
		if !validateFullName(message.Text) {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Некорректное ФИО. Пожалуйста, введите полное имя (Фамилия Имя Отчество):")
			SafeSendMessage(bot, msg)
			return
		}

		// Сохраняем ФИО и запрашиваем контакт
		state.FullName = message.Text
		state.State = "awaiting_phone"

		msg := tgbotapi.NewMessage(message.Chat.ID,
			"Спасибо! Теперь, пожалуйста, поделитесь своим контактом:")
		msg.ReplyMarkup = GetContactKeyboard()
		SafeSendMessage(bot, msg)

	case "awaiting_phone":
		// Ожидаем, что пользователь поделится контактом
		if message.Contact == nil {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Пожалуйста, нажмите кнопку 'Поделиться контактом':")
			msg.ReplyMarkup = GetContactKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		// Проверяем, что телефон принадлежит этому пользователю
		if message.Contact.UserID != message.From.ID {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Пожалуйста, поделитесь своим контактом, а не чужим:")
			msg.ReplyMarkup = GetContactKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		// Сохраняем телефон и сразу завершаем регистрацию
		state.Phone = message.Contact.PhoneNumber
		state.LocationLat = 0.0
		state.LocationLng = 0.0
		// Устанавливаем дату рождения по умолчанию (нулевая дата)
		state.BirthDate = time.Time{}

		// Пытаемся сохранить аватар пользователя
		hasAvatar := false
		err := saveUserAvatar(bot, userID)
		if err != nil {
			logger.Warning.Printf("Не удалось сохранить аватар пользователя %d: %v", userID, err)
		} else {
			// Проверяем наличие файла аватара
			avatarPath := filepath.Join("..", "uploads", "avatars", fmt.Sprintf("%d.png", userID))
			if _, err := os.Stat(avatarPath); err == nil {
				hasAvatar = true
			}
		}

		// Завершаем регистрацию
		user := &database.User{
			ID:           userID,
			FullName:     state.FullName,
			Phone:        state.Phone,
			LocationLat:  state.LocationLat,
			LocationLng:  state.LocationLng,
			BirthDate:    state.BirthDate,
			IsRegistered: true,
			HasAvatar:    hasAvatar,
		}

		err = database.UpdateUserRegistration(user)
		if err != nil {
			logger.Error.Printf("Ошибка при обновлении данных пользователя %d: %v", userID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при регистрации")
			delete(userStates, userID)
			return
		}

		// Отправляем сообщение об успешной регистрации
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"Поздравляем! Вы успешно зарегистрированы в системе поддержки.")
		msg.ReplyMarkup = GetMainMenuKeyboard()
		SafeSendMessage(bot, msg)

		// Удаляем состояние пользователя
		delete(userStates, userID)

	case "creating_ticket_category":
		// Обрабатываем категорию тикета
		if message.Text == "❌ Отмена" || message.Text == "Отмена" {
			// Отменяем создание тикета
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Создание тикета отменено.")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			delete(userStates, userID)
			return
		}

		validCategories := map[string]string{
			"💭 Вопрос":       "вопрос",
			"Вопрос":         "вопрос",
			"🚨 Важно,Срочно": "важно,срочно",
			"Важно,Срочно":   "важно,срочно",
			"💰 Финансы":      "финансы",
			"Финансы":        "финансы",
		}

		category, valid := validCategories[message.Text]
		if !valid {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Пожалуйста, выберите категорию из предложенных вариантов:")
			msg.ReplyMarkup = GetCategoryKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		state.TicketCat = category
		state.State = "creating_ticket_description"

		msg := tgbotapi.NewMessage(message.Chat.ID,
			"Пожалуйста, введите описание вашего обращения:")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
		SafeSendMessage(bot, msg)

	case "creating_ticket_description":
		// Сохраняем описание тикета
		if len(message.Text) < 10 || len(message.Text) > 1000 {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Описание должно содержать от 10 до 1000 символов. Пожалуйста, введите корректное описание:")
			SafeSendMessage(bot, msg)
			return
		}

		state.TicketDesc = message.Text

		// Автоматически генерируем заголовок тикета
		state.TicketTitle = generateTicketTitle(state.TicketCat, state.TicketDesc)

		state.State = "creating_ticket_confirm"

		// Предлагаем подтвердить создание тикета
		confirmText := fmt.Sprintf("Пожалуйста, подтвердите создание тикета:\n\n"+
			"Заголовок: %s\n"+
			"Описание: %s\n"+
			"Категория: %s\n\n"+
			"Всё верно?", state.TicketTitle, state.TicketDesc, getCategoryName(state.TicketCat))

		msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
		msg.ReplyMarkup = GetConfirmKeyboard()
		SafeSendMessage(bot, msg)

	case "creating_ticket_confirm":
		// Создаем тикет, если пользователь подтвердил
		if message.Text == "✅ Да" || message.Text == "Да" {
			// Создаем тикет в базе данных
			ticket := &database.Ticket{
				UserID:      userID,
				Title:       state.TicketTitle,
				Description: state.TicketDesc,
				Status:      "создан",
				Category:    state.TicketCat,
			}

			ticketID, err := database.CreateTicket(ticket)
			if err != nil {
				logger.Error.Printf("Ошибка при создании тикета для пользователя %d: %v", userID, err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при создании тикета")
				return
			}

			// Создаем первое сообщение в тикете
			ticketMessage := &database.TicketMessage{
				TicketID:   ticketID,
				SenderType: "user",
				SenderID:   userID,
				Message:    state.TicketDesc,
			}

			_, err = database.AddTicketMessage(ticketMessage)
			if err != nil {
				logger.Error.Printf("Ошибка при добавлении сообщения в тикет для пользователя %d: %v", userID, err)
			}

			// Отправляем сообщение об успешном создании тикета
			msg := tgbotapi.NewMessage(message.Chat.ID,
				fmt.Sprintf("🎊 Тикет #%d успешно создан! Наши специалисты свяжутся с вами в ближайшее время.", ticketID))
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)

		} else if message.Text == "❌ Нет" || message.Text == "Нет" {
			// Отменяем создание тикета
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"❌ Создание тикета отменено.")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
		} else {
			// Некорректный ответ
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Пожалуйста, выберите 'Да' или 'Нет':")
			msg.ReplyMarkup = GetConfirmKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		// Сбрасываем состояние
		delete(userStates, userID)

	case "viewing_tickets":
		// Если пользователь нажал "Назад", возвращаемся в главное меню
		if message.Text == "⬅️ Назад" {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Главное меню:")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			delete(userStates, userID)
			return
		}

		// Проверяем, нажал ли пользователь на тикет
		// Формат кнопки: "#ID статус заголовок [N сообщ.]"
		if strings.HasPrefix(message.Text, "#") {
			parts := strings.Split(message.Text, " ")
			if len(parts) < 2 {
				// Некорректный формат
				msg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось определить тикет. Пожалуйста, выберите тикет из списка:")
				SafeSendMessage(bot, msg)
				return
			}

			// Извлекаем ID тикета из текста кнопки
			ticketIDStr := parts[0][1:] // Убираем символ # в начале
			ticketID, err := strconv.Atoi(ticketIDStr)
			if err != nil {
				logger.Error.Printf("Ошибка при парсинге ID тикета: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось определить ID тикета. Пожалуйста, выберите тикет из списка:")
				SafeSendMessage(bot, msg)
				return
			}

			// Проверяем, существует ли тикет и принадлежит ли он пользователю
			ticket, err := database.GetTicketByID(ticketID)
			if err != nil || ticket.UserID != userID {
				logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Тикет не найден или вы не имеете доступа к нему.")
				SafeSendMessage(bot, msg)
				return
			}

			// Загружаем сообщения тикета
			showTicketConversation(bot, message.Chat.ID, ticketID)

			// Устанавливаем состояние просмотра конкретного тикета
			state.State = "viewing_ticket"
			state.TicketID = ticketID

			return
		}

		// Если сообщение не распознано, показываем список тикетов снова
		msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, выберите тикет из списка или нажмите 'Назад':")
		SafeSendMessage(bot, msg)

	case "viewing_ticket":
		// Если пользователь нажал "Назад", возвращаемся к списку тикетов
		if message.Text == "⬅️ Назад" {
			// Возвращаемся к списку тикетов
			HandleMainMenu(bot, &tgbotapi.Message{
				From:      message.From,
				Chat:      message.Chat,
				Text:      "Активные тикеты",
				MessageID: message.MessageID,
			})
			return
		}

		// Если пользователь нажал "Просмотреть фото"
		if message.Text == "🖼 Просмотреть фото" {
			showTicketPhotos(bot, message.Chat.ID, state.TicketID)
			return
		}

		// Проверяем, активен ли тикет
		ticket, err := database.GetTicketByID(state.TicketID)
		if err != nil {
			logger.Error.Printf("Ошибка при получении тикета %d: %v", state.TicketID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при доступе к тикету")
			return
		}

		if ticket.Status == "закрыт" {
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Тикет закрыт и не может быть обновлен.")
			SafeSendMessage(bot, msg)
			return
		}

		// Проверяем, есть ли в сообщении фотография
		if message.Photo != nil && len(message.Photo) > 0 {
			// Получаем фотографию наилучшего качества
			photo := message.Photo[len(message.Photo)-1]

			// Получаем путь к директории uploads
			uploadsDir := ensureUploadsDir()

			// Создаем директорию для пользователя, если нужно
			userDir := filepath.Join(uploadsDir, strconv.FormatInt(userID, 10))
			if _, err := os.Stat(userDir); os.IsNotExist(err) {
				err := os.MkdirAll(userDir, 0755)
				if err != nil {
					logger.Error.Printf("Ошибка при создании директории пользователя: %v", err)
					SendErrorMessage(bot, message.Chat.ID, "Ошибка при сохранении фотографии")
					return
				}
			}

			// Создаем директорию для тикета, если нужно
			ticketDir := filepath.Join(userDir, strconv.Itoa(state.TicketID))
			if _, err := os.Stat(ticketDir); os.IsNotExist(err) {
				err := os.MkdirAll(ticketDir, 0755)
				if err != nil {
					logger.Error.Printf("Ошибка при создании директории тикета: %v", err)
					SendErrorMessage(bot, message.Chat.ID, "Ошибка при сохранении фотографии")
					return
				}
			}

			// Получаем файл
			fileURL, err := bot.GetFileDirectURL(photo.FileID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении URL фото: %v", err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при загрузке фотографии")
				return
			}

			// Определяем расширение файла из URL или Content-Type
			resp, err := http.Get(fileURL)
			if err != nil {
				logger.Error.Printf("Ошибка при скачивании фото: %v", err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при загрузке фотографии")
				return
			}
			defer resp.Body.Close()

			// Определяем расширение файла из Content-Type
			contentType := resp.Header.Get("Content-Type")
			ext := ".jpg" // По умолчанию
			switch contentType {
			case "image/jpeg", "image/jpg":
				ext = ".jpg"
			case "image/png":
				ext = ".png"
			case "image/gif":
				ext = ".gif"
			case "image/webp":
				ext = ".webp"
			}

			// Генерируем имя файла и полный путь
			fileName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), photo.FileID, ext)
			filePath := filepath.Join(ticketDir, fileName)

			// Создаем файл для сохранения
			file, err := os.Create(filePath)
			if err != nil {
				logger.Error.Printf("Ошибка при создании файла: %v", err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при сохранении фотографии")
				return
			}
			defer file.Close()

			// Копируем содержимое
			_, err = io.Copy(file, resp.Body)
			if err != nil {
				logger.Error.Printf("Ошибка при сохранении фото: %v", err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при сохранении фотографии")
				return
			}

			// Добавляем сообщение о прикреплении фото
			messageText := fmt.Sprintf("прикрепил фото %s", fileName)
			ticketMessage := &database.TicketMessage{
				TicketID:   state.TicketID,
				SenderType: "user",
				SenderID:   userID,
				Message:    messageText,
			}

			// Получаем ID сообщения после его добавления
			messageID, err := database.AddTicketMessage(ticketMessage)
			if err != nil {
				logger.Error.Printf("Ошибка при добавлении сообщения в тикет %d: %v", state.TicketID, err)
				SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при отправке сообщения")
				return
			}

			// Сохраняем информацию о фотографии в базу данных
			ticketPhoto := &database.TicketPhoto{
				TicketID:   state.TicketID,
				SenderType: "user",
				SenderID:   userID,
				FilePath:   filePath,
				FileID:     photo.FileID,
				MessageID:  messageID, // Используем полученный ID сообщения
			}

			_, err = database.AddTicketPhoto(ticketPhoto)
			if err != nil {
				logger.Error.Printf("Ошибка при сохранении информации о фото: %v", err)
			}

			// Обновляем статус тикета на "ожидает ответа"
			err = database.UpdateTicketStatus(state.TicketID, "Ожидает действий поддержки")
			if err != nil {
				logger.Error.Printf("Ошибка при обновлении статуса тикета %d: %v", state.TicketID, err)
			}

			// Подтверждаем отправку фото
			msg := tgbotapi.NewMessage(message.Chat.ID,
				"✅ Ваша фотография успешно прикреплена к тикету.")
			SafeSendMessage(bot, msg)

			// Показываем обновленный диалог
			showTicketConversation(bot, message.Chat.ID, state.TicketID)
			return
		}

		// Добавляем сообщение пользователя в тикет
		ticketMessage := &database.TicketMessage{
			TicketID:   state.TicketID,
			SenderType: "user",
			SenderID:   userID,
			Message:    message.Text,
		}

		// Если пользователь нажал "Закрыть тикет"
		if message.Text == "❌ Закрыть тикет" {
			// Закрываем тикет
			err := database.CloseTicket(state.TicketID, userID)
			if err != nil {
				logger.Error.Printf("Ошибка при закрытии тикета %d: %v", state.TicketID, err)
				SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("Не удалось закрыть тикет: %v", err))
				return
			}

			msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Тикет успешно закрыт")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			delete(userStates, userID)
			return
		}

		messageID, err := database.AddTicketMessage(ticketMessage)
		if err != nil {
			logger.Error.Printf("Ошибка при добавлении сообщения в тикет %d: %d %v", state.TicketID, messageID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при отправке сообщения")
			return
		}

		// Обновляем статус тикета на "ожидает ответа"
		err = database.UpdateTicketStatus(state.TicketID, "ожидает ответа")
		if err != nil {
			logger.Error.Printf("Ошибка при обновлении статуса тикета %d: %v", state.TicketID, err)
		}

		// Отправляем уведомление об успешной отправке сообщения
		successMsg := tgbotapi.NewMessage(message.Chat.ID, "🎉 Ваше сообщение успешно отправлено!")
		SafeSendMessage(bot, successMsg)

		// Показываем обновленный диалог
		showTicketConversation(bot, message.Chat.ID, state.TicketID)
		return

		// Модифицируем функцию для добавления кнопки просмотра фото
		if ticket.Status != "закрыт" {
			// Клавиатура с кнопками "Назад" и "Просмотреть фото"
			keyboard := tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("🖼 Просмотреть фото"),
				),
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("⬅️ Назад"),
				),
			)

			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Чтобы ответить, просто напишите сообщение или прикрепите фотографию.\n"+
					"Для просмотра прикрепленных фотографий нажмите 'Просмотреть фото'.\n"+
					"Для возврата в меню нажмите 'Назад'")

			msg.ReplyMarkup = keyboard
			SafeSendMessage(bot, msg)
		} else {
			// Если тикет закрыт, показываем только кнопки просмотра и возврата
			keyboard := tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("🖼 Просмотреть фото"),
				),
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("⬅️ Назад"),
				),
			)

			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Тикет закрыт и не может быть обновлен.\n"+
					"Вы можете просмотреть прикрепленные фотографии или вернуться в меню.")

			msg.ReplyMarkup = keyboard
			SafeSendMessage(bot, msg)
		}

	case "viewing_history":
		// Если пользователь нажал "Назад", возвращаемся в главное меню
		if message.Text == "⬅️ Назад" {
			// Возвращаемся в главное меню
			msg := tgbotapi.NewMessage(message.Chat.ID, "🏠 Главное меню")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			delete(userStates, userID)
			return
		}

		// Проверяем, нажал ли пользователь на тикет из истории
		if strings.HasPrefix(message.Text, "#") {
			parts := strings.Split(message.Text, " ")
			if len(parts) < 2 {
				msg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось определить тикет. Пожалуйста, выберите тикет из списка:")
				SafeSendMessage(bot, msg)
				return
			}

			// Извлекаем ID тикета из текста кнопки
			ticketIDStr := parts[0][1:] // Убираем символ # в начале
			ticketID, err := strconv.Atoi(ticketIDStr)
			if err != nil {
				logger.Error.Printf("Ошибка при парсинге ID тикета: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось определить ID тикета. Пожалуйста, выберите тикет из списка:")
				SafeSendMessage(bot, msg)
				return
			}

			// Проверяем, существует ли тикет и принадлежит ли он пользователю
			ticket, err := database.GetTicketByID(ticketID)
			if err != nil || ticket.UserID != userID {
				logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Тикет не найден или вы не имеете доступа к нему.")
				SafeSendMessage(bot, msg)
				return
			}

			// Загружаем сообщения тикета в режиме только для чтения
			showTicketConversationReadOnly(bot, message.Chat.ID, ticketID)

			// Устанавливаем состояние просмотра конкретного тикета из истории
			state.State = "viewing_history_ticket"
			state.TicketID = ticketID

			return
		}

		// Если сообщение не распознано, показываем список тикетов снова
		msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, выберите тикет из списка или нажмите 'Назад':")
		SafeSendMessage(bot, msg)

	case "viewing_history_ticket":
		// Если пользователь нажал "Назад", возвращаемся к истории тикетов
		if message.Text == "⬅️ Назад" {
			// Возвращаемся к истории тикетов
			HandleMainMenu(bot, &tgbotapi.Message{
				From:      message.From,
				Chat:      message.Chat,
				Text:      "История тикетов",
				MessageID: message.MessageID,
			})
			return
		}

		// Если пользователь нажал "Просмотреть фото"
		if message.Text == "🖼 Просмотреть фото" {
			showTicketPhotos(bot, message.Chat.ID, state.TicketID)
			return
		}

		// В режиме просмотра истории нельзя отправлять сообщения
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"📖 Этот тикет открыт только для просмотра.\n\n"+
				"🖼 Вы можете просмотреть прикрепленные фотографии\n"+
				"⬅️ Или вернуться к истории тикетов")
		SafeSendMessage(bot, msg)

	// Другие состояния могут быть добавлены по мере необходимости
	default:
		// По умолчанию проверяем, зарегистрирован ли пользователь
		isRegistered, err := database.IsUserRegistered(userID)
		if err != nil {
			logger.Error.Printf("Ошибка при проверке регистрации %d: %v", userID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при проверке регистрации")
			return
		}

		if isRegistered {
			// Обрабатываем сообщение как команду в главном меню
			HandleMainMenu(bot, message)
		} else {
			// Начинаем процесс регистрации
			userStates[userID] = &UserState{State: "awaiting_fullname"}

			msg := tgbotapi.NewMessage(message.Chat.ID,
				"Для начала работы необходимо зарегистрироваться.\n\n"+
					"Пожалуйста, введите ваше полное имя (Фамилия Имя Отчество):")
			SafeSendMessage(bot, msg)
		}
	}
}
