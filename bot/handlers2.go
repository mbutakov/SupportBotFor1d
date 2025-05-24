package bot

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"supportTicketBotGo/database"
	"supportTicketBotGo/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skip2/go-qrcode"
)

// Обработчик сообщений в главном меню
func HandleMainMenu(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userID := message.From.ID

	switch message.Text {
	case "🎯 Активные тикеты", "Активные тикеты":
		tickets, err := database.GetActiveTicketsByUserID(userID)
		if err != nil {
			logger.Error.Printf("Ошибка при получении активных тикетов пользователя %d: %v", userID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при получении тикетов")
			return
		}

		if len(tickets) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "📭 У вас нет активных тикетов.")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		// Создаем клавиатуру с тикетами
		ticketButtons := make([][]tgbotapi.KeyboardButton, 0, len(tickets)+1)

		for _, ticket := range tickets {
			// Получаем количество сообщений в тикете
			count, err := database.GetTicketMessageCount(ticket.ID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении количества сообщений тикета %d: %v", ticket.ID, err)
				count = 0
			}

			// Определяем эмодзи статуса
			statusEmoji := getStatusEmoji(ticket.Status)

			// Создаем кнопку с информацией о тикете
			buttonLabel := fmt.Sprintf("#%d %s %s [%d сообщ.]",
				ticket.ID, statusEmoji, truncateString(ticket.Title, 25), count)

			ticketButtons = append(ticketButtons, tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(buttonLabel),
			))
		}

		// Добавляем кнопку "Назад"
		ticketButtons = append(ticketButtons, tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Назад"),
		))

		msg := tgbotapi.NewMessage(message.Chat.ID, "Ваши активные тикеты:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(ticketButtons...)
		SafeSendMessage(bot, msg)

		// Устанавливаем состояние просмотра тикетов
		userStates[userID] = &UserState{State: "viewing_tickets"}

	case "📚 История тикетов", "История тикетов":
		tickets, err := database.GetTicketHistory(userID)
		if err != nil {
			logger.Error.Printf("Ошибка при получении истории тикетов пользователя %d: %v", userID, err)
			SendErrorMessage(bot, message.Chat.ID, "Произошла ошибка при получении истории тикетов")
			return
		}

		if len(tickets) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "📚 У вас пока нет тикетов.")
			msg.ReplyMarkup = GetMainMenuKeyboard()
			SafeSendMessage(bot, msg)
			return
		}

		// Ограничиваем количество тикетов до 50
		maxTickets := 50
		var ticketsToShow []database.Ticket
		var limitMessage string

		if len(tickets) > maxTickets {
			ticketsToShow = tickets[len(tickets)-maxTickets:]
			limitMessage = fmt.Sprintf("⚠️ Показаны последние %d тикетов из всей истории.", maxTickets)
		} else {
			ticketsToShow = tickets
			limitMessage = ""
		}

		// Отправляем заголовок истории тикетов
		headerMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("📚 *История ваших тикетов* (%d)\n\n%s", len(tickets), limitMessage))
		headerMsg.ParseMode = "Markdown"
		headerMsg.ReplyMarkup = GetMainMenuKeyboard()
		SafeSendMessage(bot, headerMsg)

		// Отправляем каждый тикет как отдельное сообщение
		for _, ticket := range ticketsToShow {
			// Получаем количество сообщений в тикете
			count, err := database.GetTicketMessageCount(ticket.ID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении количества сообщений тикета %d: %v", ticket.ID, err)
				count = 0
			}

			// Определяем эмодзи статуса
			statusEmoji := getStatusEmoji(ticket.Status)

			// Форматируем дату создания
			createdDate := ticket.CreatedAt.Format("02.01.2006 15:04")

			// Форматируем дату закрытия, если тикет закрыт
			closedDate := ""
			if ticket.Status == "закрыт" && ticket.ClosedAt.Valid {
				closedDate = fmt.Sprintf("\n🔒 Закрыт: %s", ticket.ClosedAt.Time.Format("02.01.2006 15:04"))
			}

			// Создаем сообщение с информацией о тикете
			ticketMsg := fmt.Sprintf(
				"🔖 *Тикет #%d*\n%s %s\n\n📝 Категория: %s\n📅 Создан: %s%s\n💬 Сообщений: %d\n\nЧтобы просмотреть этот тикет, отправьте команду:\n`/ticket %d`",
				ticket.ID,
				statusEmoji,
				ticket.Title,
				ticket.Category,
				createdDate,
				closedDate,
				count,
				ticket.ID,
			)

			msg := tgbotapi.NewMessage(message.Chat.ID, ticketMsg)
			msg.ParseMode = "Markdown"
			SafeSendMessage(bot, msg)

			// Добавляем небольшую задержку между сообщениями, чтобы избежать флуда
			time.Sleep(100 * time.Millisecond)
		}

		// Устанавливаем состояние просмотра истории тикетов
		userStates[userID] = &UserState{State: "main_menu"}

	case "✨ Создать тикет", "Создать тикет":
		// Начинаем процесс создания тикета с выбора категории
		userStates[userID] = &UserState{State: "creating_ticket_category"}

		msg := tgbotapi.NewMessage(message.Chat.ID,
			"🎯 Выберите категорию обращения:")
		msg.ReplyMarkup = GetCategoryKeyboard()
		SafeSendMessage(bot, msg)

	default:
		// Если команда не распознана, показываем главное меню
		msg := tgbotapi.NewMessage(message.Chat.ID,
			"Пожалуйста, выберите действие из меню:")
		msg.ReplyMarkup = GetMainMenuKeyboard()
		SafeSendMessage(bot, msg)
	}
}

// Функция для создания заголовка из категории и описания
func generateTicketTitle(category, description string) string {
	// Конвертируем категорию для заголовка
	categoryPrefix := map[string]string{
		"спросить": "Вопрос",
		"проблема": "Проблема",
		"идея":     "Идея",
	}[category]

	// Берем первые 4 слова из описания
	words := strings.Fields(description)
	maxWords := 4
	if len(words) < maxWords {
		maxWords = len(words)
	}

	shortDescription := strings.Join(words[:maxWords], " ")

	// Если описание слишком длинное, добавляем многоточие
	if len(words) > maxWords {
		shortDescription += "..."
	}

	return fmt.Sprintf("%s: %s", categoryPrefix, shortDescription)
}

// getStatusEmoji возвращает эмодзи статуса тикета (внутренняя функция)
func getStatusEmoji(status string) string {
	return GetStatusEmoji(status)
}

// GetStatusEmoji возвращает эмодзи статуса тикета (экспортируемая функция)
func GetStatusEmoji(status string) string {
	switch status {
	case "открыт":
		return "🌟" // Новый тикет
	case "в работе":
		return "🔄" // Тикет в работе
	case "ожидает ответа":
		return "💭" // Ожидает ответа
	case "закрыт":
		return "✨" // Закрытый тикет
	default:
		return "❓" // Неизвестный статус
	}
}

// truncateString обрезает строку до указанной длины и добавляет многоточие если нужно
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

// showTicketConversationReadOnly отображает все сообщения тикета в режиме только для чтения
func showTicketConversationReadOnly(bot *tgbotapi.BotAPI, chatID int64, ticketID int) {
	// Получаем информацию о тикете
	ticket, err := database.GetTicketByID(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить информацию о тикете")
		return
	}

	// Получаем все сообщения тикета
	messages, err := database.GetTicketMessages(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении сообщений тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить сообщения тикета")
		return
	}

	// Ограничиваем количество сообщений
	maxMessages := 50
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
		warningMsg := tgbotapi.NewMessage(chatID,
			fmt.Sprintf("⚠️ Показаны только последние %d сообщений из %d", maxMessages, len(messages)))
		SafeSendMessage(bot, warningMsg)
	}

	// Формируем красивую шапку тикета с эмодзи и пометкой "только для чтения"
	headerText := fmt.Sprintf("📖 *ТИКЕТ #%d (ТОЛЬКО ПРОСМОТР)* 📖\n\n"+
		"📝 *Тема:* %s\n"+
		"📅 *Создан:* %s\n"+
		"🏷️ *Категория:* %s\n"+
		"📊 *Статус:* %s %s\n\n"+
		"💬 *ИСТОРИЯ ДИАЛОГА:*\n",
		ticket.ID, ticket.Title,
		ticket.CreatedAt.Format("02.01.2006 15:04"),
		getCategoryName(ticket.Category),
		getStatusEmoji(ticket.Status), ticket.Status)

	// Объединяем сообщения в более крупные блоки
	const maxTelegramMessageSize = 4000
	var combinedMessages = headerText
	var messageCounter int

	// Если нет сообщений, покажем это в первом блоке
	if len(messages) == 0 {
		combinedMessages += "\n🔍 В этом тикете пока нет сообщений."
		msg := tgbotapi.NewMessage(chatID, combinedMessages)
		msg.ParseMode = "Markdown"
		SafeSendMessage(bot, msg)
	} else {
		// Собираем сообщения в блоки
		for i, message := range messages {
			var sender string
			var messagePrefix string
			var senderEmoji string

			if message.SenderType == "user" {
				senderEmoji = "👤"
				sender = "Вы"
				messagePrefix = "💬"
			} else {
				senderEmoji = "👨‍💼"
				// Получаем имя сотрудника поддержки
				supportName, err := database.GetUserNameByID(message.SenderID)
				if err != nil {
					supportName = "Поддержка"
				}
				sender = supportName
				messagePrefix = "🗨️"
			}

			// Формируем текст одного сообщения с эмодзи
			timeStr := message.CreatedAt.Format("02.01.2006 15:04")
			singleMessage := fmt.Sprintf("%s\n%s %s (%s)\n%s %s\n",
				getSeparator(messageCounter > 0),
				senderEmoji, sender,
				timeStr,
				messagePrefix,
				formatMessageText(message.Message))

			// Если это первое сообщение или добавление превысит лимит
			if messageCounter == 0 || len(combinedMessages)+len(singleMessage) > maxTelegramMessageSize {
				if messageCounter > 0 {
					// Отправляем предыдущий блок
					msg := tgbotapi.NewMessage(chatID, combinedMessages)
					msg.ParseMode = "Markdown"
					SafeSendMessage(bot, msg)
				}

				// Если это первое сообщение в новом блоке (не первый блок)
				if messageCounter > 0 {
					// Новый блок начинаем с заголовка продолжения
					combinedMessages = fmt.Sprintf("📖 *ТИКЕТ #%d (ТОЛЬКО ПРОСМОТР - продолжение)* 📖\n\n", ticket.ID)
				}

				// Добавляем текущее сообщение
				combinedMessages += singleMessage
				messageCounter = 1
			} else {
				// Добавляем сообщение к текущему блоку
				combinedMessages += singleMessage
				messageCounter++
			}

			// Для последнего сообщения
			if i == len(messages)-1 && combinedMessages != "" {
				msg := tgbotapi.NewMessage(chatID, combinedMessages)
				msg.ParseMode = "Markdown"
				SafeSendMessage(bot, msg)
			}
		}
	}

	// Показываем только кнопки для просмотра фото и возврата
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🖼 Просмотреть фото"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Назад"),
		),
	)

	msg := tgbotapi.NewMessage(chatID,
		"📖 *Режим просмотра (только чтение)*\n\n"+
			"🖼 Для просмотра фотографий нажмите 'Просмотреть фото'\n"+
			"⬅️ Для возврата к истории тикетов нажмите 'Назад'")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	SafeSendMessage(bot, msg)
}

// showTicketConversation отображает все сообщения тикета
func showTicketConversation(bot *tgbotapi.BotAPI, chatID int64, ticketID int) {
	// Получаем информацию о тикете
	ticket, err := database.GetTicketByID(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить информацию о тикете")
		return
	}

	// Получаем все сообщения тикета
	messages, err := database.GetTicketMessages(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении сообщений тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить сообщения тикета")
		return
	}

	// Ограничиваем количество сообщений
	maxMessages := 50
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
		warningMsg := tgbotapi.NewMessage(chatID,
			fmt.Sprintf("⚠️ Показаны только последние %d сообщений из %d", maxMessages, len(messages)))
		SafeSendMessage(bot, warningMsg)
	}

	// Формируем красивую шапку тикета с эмодзи
	headerText := fmt.Sprintf("🎫 *ТИКЕТ #%d* 🎫\n\n"+
		"📝 *Тема:* %s\n"+
		"📅 *Создан:* %s\n"+
		"🏷️ *Категория:* %s\n"+
		"📊 *Статус:* %s %s\n\n"+
		"💬 *ИСТОРИЯ ДИАЛОГА:*\n",
		ticket.ID, ticket.Title,
		ticket.CreatedAt.Format("02.01.2006 15:04"),
		getCategoryName(ticket.Category),
		getStatusEmoji(ticket.Status), ticket.Status)

	// Объединяем сообщения в более крупные блоки
	const maxTelegramMessageSize = 4000 // Немного меньше максимального (4096), для запаса
	var combinedMessages = headerText   // Начинаем с шапки
	var messageCounter int

	// Если нет сообщений, покажем это в первом блоке
	if len(messages) == 0 {
		combinedMessages += "\n🔍 В этом тикете пока нет сообщений."
		msg := tgbotapi.NewMessage(chatID, combinedMessages)
		msg.ParseMode = "Markdown"
		SafeSendMessage(bot, msg)
	} else {
		// Собираем сообщения в блоки
		for i, message := range messages {
			var sender string
			var messagePrefix string
			var senderEmoji string

			if message.SenderType == "user" {
				senderEmoji = "👤"
				sender = "Вы"
				messagePrefix = "💬"
			} else {
				senderEmoji = "👨‍💼"
				// Получаем имя сотрудника поддержки
				supportName, err := database.GetUserNameByID(message.SenderID)
				if err != nil {
					supportName = "Поддержка"
				}
				sender = supportName
				messagePrefix = "🗨️"
			}

			// Формируем текст одного сообщения с эмодзи
			timeStr := message.CreatedAt.Format("02.01.2006 15:04")
			singleMessage := fmt.Sprintf("%s\n%s %s (%s)\n%s %s\n",
				getSeparator(messageCounter > 0), // Разделитель между сообщениями
				senderEmoji, sender,              // Имя отправителя с эмодзи
				timeStr,
				messagePrefix,
				formatMessageText(message.Message)) // Текст в копируемом блоке

			// Если это первое сообщение или добавление превысит лимит
			if messageCounter == 0 || len(combinedMessages)+len(singleMessage) > maxTelegramMessageSize {
				if messageCounter > 0 {
					// Отправляем предыдущий блок
					msg := tgbotapi.NewMessage(chatID, combinedMessages)
					msg.ParseMode = "Markdown"
					SafeSendMessage(bot, msg)
				}

				// Если это первое сообщение в новом блоке (не первый блок)
				if messageCounter > 0 {
					// Новый блок начинаем с заголовка продолжения
					combinedMessages = fmt.Sprintf("🎫 *ТИКЕТ #%d (продолжение)* 🎫\n\n", ticket.ID)
				}

				// Добавляем текущее сообщение
				combinedMessages += singleMessage
				messageCounter = 1
			} else {
				// Добавляем сообщение к текущему блоку
				combinedMessages += singleMessage
				messageCounter++
			}

			// Для последнего сообщения
			if i == len(messages)-1 && combinedMessages != "" {
				msg := tgbotapi.NewMessage(chatID, combinedMessages)
				msg.ParseMode = "Markdown"
				SafeSendMessage(bot, msg)
			}
		}
	}

	// Предлагаем ответить на тикет
	if ticket.Status != "закрыт" {
		// Клавиатура с кнопками
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("🖼 Просмотреть фото"),
				tgbotapi.NewKeyboardButton("❌ Закрыть тикет"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⬅️ Назад"),
			),
		)

		msg := tgbotapi.NewMessage(chatID,
			"✏️ *Чтобы ответить, просто напишите сообщение или прикрепите фотографию.*\n\n"+
				"🖼 Для просмотра фотографий нажмите 'Просмотреть фото'\n"+
				"⬅️ Для возврата в меню нажмите 'Назад'")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		SafeSendMessage(bot, msg)
	} else {
		// Если тикет закрыт
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("🖼 Просмотреть фото"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⬅️ Назад"),
			),
		)

		msg := tgbotapi.NewMessage(chatID,
			"🔒 *Тикет закрыт и не может быть обновлен.*\n\n"+
				"🖼 Вы можете просмотреть прикрепленные фотографии\n"+
				"⬅️ Или вернуться в главное меню")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		SafeSendMessage(bot, msg)
	}
}

// Более эстетичный разделитель сообщений
func getSeparator(show bool) string {
	if show {
		return "\n┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄"
	}
	return ""
}

// Функция для оформления текста сообщения в копируемый блок
func formatMessageText(text string) string {
	// Блок кода в три обратные кавычки - подходит для копирования
	return "`\n" + text + "`"
}

// getCategoryName возвращает название категории
func getCategoryName(category string) string {
	switch category {
	case "Вопрос":
		return "Вопрос"
	case "Важное":
		return "Важно,Срочно"
	case "Финансы":
		return "Финансы"
	default:
		return category
	}
}

// HandleCloseTicket обрабатывает закрытие тикета
func HandleCloseTicket(bot *tgbotapi.BotAPI, chatID int64, userID int64, ticketID int) {
	// Закрываем тикет в базе данных
	err := database.CloseTicket(ticketID, userID)
	if err != nil {
		logger.Error.Printf("Ошибка при закрытии тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось закрыть тикет. Пожалуйста, попробуйте позже.")
		return
	}

	// Отправляем сообщение об успешном закрытии
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🔒 *Тикет #%d успешно закрыт*\n\nСпасибо за обращение! Если у вас появятся новые вопросы, вы всегда можете создать новый тикет.", ticketID))
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = GetMainMenuKeyboard()
	SafeSendMessage(bot, msg)

	// Обновляем состояние пользователя
	userStates[userID] = &UserState{State: "main_menu"}
}

// Добавляем новую функцию для отправки случайных советов
func SendRandomTip(bot *tgbotapi.BotAPI, chatID int64) {
	tips := []string{
		"💡 Совет: Прикрепляйте фотографии к тикетам для более быстрого решения проблемы.",
		"💡 Совет: Подробно описывайте проблему в тикете для более эффективной помощи.",
		"💡 Совет: Используйте категорию 'Важно,Срочно' только для действительно срочных вопросов.",
		"💡 Совет: Проверяйте статус ваших тикетов регулярно для получения обновлений.",
		"💡 Совет: Если проблема решена, не забудьте закрыть тикет.",
	}

	// Выбираем случайный совет
	rand.Seed(time.Now().UnixNano())
	randomTip := tips[rand.Intn(len(tips))]

	// Отправляем совет
	msg := tgbotapi.NewMessage(chatID, randomTip)
	SafeSendMessage(bot, msg)
}

// showTicketPhotos отображает все фотографии тикета
func showTicketPhotos(bot *tgbotapi.BotAPI, chatID int64, ticketID int) {
	// Получаем все фотографии тикета
	photos, err := database.GetTicketPhotos(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении фотографий тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить фотографии тикета")
		return
	}

	if len(photos) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📷 В этом тикете нет прикрепленных фотографий.")
		SafeSendMessage(bot, msg)
		return
	}

	// Отправляем сообщение с количеством фотографий
	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("🖼 *Фотографии к тикету #%d*\n\nНайдено фотографий: %d", ticketID, len(photos)))
	msg.ParseMode = "Markdown"
	SafeSendMessage(bot, msg)

	// Отправляем каждую фотографию (максимум 10)
	maxPhotos := 10
	if len(photos) > maxPhotos {
		photos = photos[len(photos)-maxPhotos:]
		warningMsg := tgbotapi.NewMessage(chatID,
			fmt.Sprintf("⚠️ Показаны только последние %d из %d фотографий", maxPhotos, len(photos)))
		SafeSendMessage(bot, warningMsg)
	}

	for i, photo := range photos {
		// Вместо использования FileID, прочитаем файл с диска
		var photoMsg tgbotapi.PhotoConfig

		// Проверяем, существует ли файл
		_, err := os.Stat(photo.FilePath)
		if err != nil {
			// Если файл не найден, отправляем сообщение о недоступности
			errorMsg := tgbotapi.NewMessage(chatID,
				fmt.Sprintf("⚠️ Фото #%d недоступно: файл не найден", i+1))
			SafeSendMessage(bot, errorMsg)
			continue
		}

		// Открываем файл
		file, err := os.Open(photo.FilePath)
		if err != nil {
			logger.Error.Printf("Ошибка при открытии файла %s: %v", photo.FilePath, err)
			errorMsg := tgbotapi.NewMessage(chatID,
				fmt.Sprintf("⚠️ Фото #%d недоступно: ошибка чтения файла", i+1))
			SafeSendMessage(bot, errorMsg)
			continue
		}
		defer file.Close()

		// Создаем новую фотографию из файла
		photoMsg = tgbotapi.NewPhoto(chatID, tgbotapi.FileReader{
			Name:   fmt.Sprintf("photo_%d.jpg", i+1),
			Reader: file,
		})

		var senderEmoji, sender string
		if photo.SenderType == "user" {
			senderEmoji = "👤"
			sender = "Вы"
		} else {
			senderEmoji = "👨‍💼"
			supportName, err := database.GetUserNameByID(photo.SenderID)
			if err != nil {
				supportName = "Поддержка"
			}
			sender = supportName
		}

		photoMsg.Caption = fmt.Sprintf("📷 *Фото #%d*\n👤 Отправитель: %s %s\n🕒 Дата: %s",
			i+1, senderEmoji, sender, photo.CreatedAt.Format("02.01.2006 15:04"))
		photoMsg.ParseMode = "Markdown"

		// Отправляем фото
		_, err = bot.Send(photoMsg)
		if err != nil {
			logger.Error.Printf("Ошибка при отправке фото %s: %v", photo.FilePath, err)
			errorMsg := tgbotapi.NewMessage(chatID,
				fmt.Sprintf("⚠️ Не удалось отправить фото #%d", i+1))
			SafeSendMessage(bot, errorMsg)
		}

		// Задержка между отправкой фотографий
		time.Sleep(300 * time.Millisecond)
	}

	// Отправляем кнопку "Назад"
	backMsg := tgbotapi.NewMessage(chatID, "⬅️ Для возврата к диалогу нажмите 'Назад'")
	backMsg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⬅️ Назад"),
		),
	)
	SafeSendMessage(bot, backMsg)
}

// Добавляем новую функцию для генерации QR-кода с информацией о тикете
func generateTicketQR(bot *tgbotapi.BotAPI, chatID int64, ticketID int) {
	// Получаем информацию о тикете
	ticket, err := database.GetTicketByID(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить информацию о тикете")
		return
	}

	// Формируем текст для QR-кода
	qrText := fmt.Sprintf("Тикет #%d\nТема: %s\nСтатус: %s\nСоздан: %s",
		ticket.ID, ticket.Title, ticket.Status, ticket.CreatedAt.Format("02.01.2006 15:04"))

	// Генерируем QR-код
	// Используем helper для получения/создания директории ../uploads
	qrDirBase := ensureUploadsDir()

	// Путь к файлу QR-кода
	// ensureUploadsDir обрабатывает создание базовой директории.
	// Если произошла ошибка в ensureUploadsDir и она вернула "uploads",
	// QR-код будет сохранен в локальной директории "uploads".
	qrFilePath := filepath.Join(qrDirBase, fmt.Sprintf("qr_ticket_%d.png", ticketID))
	err = qrcode.WriteFile(qrText, qrcode.Medium, 256, qrFilePath)
	if err != nil {
		logger.Error.Printf("Ошибка при генерации QR-кода: %v", err)
		SendErrorMessage(bot, chatID, "Не удалось сгенерировать QR-код")
		return
	}

	// Отправляем QR-код пользователю
	file, err := os.Open(qrFilePath)
	if err != nil {
		logger.Error.Printf("Ошибка при открытии файла QR-кода: %v", err)
		SendErrorMessage(bot, chatID, "Не удалось отправить QR-код")
		return
	}
	defer file.Close()

	photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileReader{
		Name:   fmt.Sprintf("qr_ticket_%d.png", ticketID),
		Reader: file,
	})
	photoMsg.Caption = "🔍 QR-код с информацией о вашем тикете"

	_, err = bot.Send(photoMsg)
	if err != nil {
		logger.Error.Printf("Ошибка при отправке QR-кода: %v", err)
		SendErrorMessage(bot, chatID, "Не удалось отправить QR-код")
	}
}

// Добавляем новую функцию для отображения статуса тикета
func showTicketStatus(bot *tgbotapi.BotAPI, chatID int64, ticketID int) {
	// Получаем информацию о тикете
	ticket, err := database.GetTicketByID(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
		SendErrorMessage(bot, chatID, "Не удалось загрузить информацию о тикете")
		return
	}

	// Получаем количество сообщений
	messageCount, err := database.GetTicketMessageCount(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении количества сообщений: %v", err)
		messageCount = 0
	}

	// Получаем количество фотографий
	photos, err := database.GetTicketPhotos(ticketID)
	if err != nil {
		logger.Error.Printf("Ошибка при получении фотографий: %v", err)
		photos = nil
	}

	// Формируем красивое сообщение о статусе
	statusText := fmt.Sprintf("📊 *Статус тикета #%d*\n\n"+
		"📝 *Тема:* %s\n"+
		"🏷️ *Категория:* %s\n"+
		"📊 *Текущий статус:* %s %s\n"+
		"📅 *Создан:* %s\n"+
		"💬 *Сообщений:* %d\n"+
		"🖼 *Фотографий:* %d\n\n"+
		"⏱ *Время последнего обновления:* %s",
		ticket.ID, ticket.Title,
		getCategoryName(ticket.Category),
		getStatusEmoji(ticket.Status), ticket.Status,
		ticket.CreatedAt.Format("02.01.2006 15:04"),
		messageCount,
		len(photos),
		time.Now().Format("02.01.2006 15:04:05"))

	msg := tgbotapi.NewMessage(chatID, statusText)
	msg.ParseMode = "Markdown"
	SafeSendMessage(bot, msg)
}
