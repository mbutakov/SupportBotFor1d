package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"supportTicketBotGo/bot"
	"supportTicketBotGo/config"
	"supportTicketBotGo/database"
	"supportTicketBotGo/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Парсим флаги командной строки
	configPath := flag.String("config", "config.json", "Путь к конфигурационному файлу")
	webhookHost := flag.String("webhook", "", "URL для webhook (например, https://example.com)")
	port := flag.String("port", "8443", "Порт для webhook сервера")
	flag.Parse()

	// Загружаем конфигурацию
	err := config.LoadConfig(*configPath)
	if err != nil {
		panic("Ошибка загрузки конфигурации: " + err.Error())
	}

	// Инициализируем логер
	err = logger.InitLogger(config.AppConfig.LogFile)
	if err != nil {
		panic("Ошибка инициализации логера: " + err.Error())
	}
	logger.Info.Println("Логер инициализирован")

	// Подключаемся к базе данных
	err = database.ConnectDBOptimized()
	if err != nil {
		logger.Error.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	logger.Info.Println("Подключение к базе данных установлено")

	// Инициализируем Telegram бота
	botAPI, err := tgbotapi.NewBotAPI(config.AppConfig.TelegramToken)
	if err != nil {
		logger.Error.Fatalf("Ошибка инициализации Telegram бота: %v", err)
	}

	// В режиме разработки можно включить отладочный вывод
	// botAPI.Debug = true

	logger.Info.Printf("Авторизован как %s", botAPI.Self.UserName)

	// Канал для перехвата сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Флаг для корректного завершения работы бота
	isRunning := true
	var wg sync.WaitGroup

	// Определяем режим работы: webhook или long polling
	if *webhookHost != "" {
		// Режим webhook
		// *webhookHost должен быть вашим публичным доменом с протоколом, например, "https://mb0.tech"
		// *port - это внутренний порт, на котором слушает Go приложение, например, "8443"

		// Формируем публичный URL, который будет вызван Telegram
		publicWebhookURL := *webhookHost + "/webhook/" + config.AppConfig.SecureWebhookToken
		logger.Info.Printf("Публичный URL для Telegram webhook: %s", publicWebhookURL)

		// Настраиваем webhook для Telegram
		whConfig, err := tgbotapi.NewWebhook(publicWebhookURL) // Используем publicWebhookURL
		if err != nil {
			logger.Error.Fatalf("Ошибка при создании конфигурации webhook: %v", err)
		}
		_, err = botAPI.Request(whConfig) // Отправляем запрос на установку вебхука
		if err != nil {
			logger.Error.Fatalf("Ошибка при установке webhook: %v", err)
		}

		// Получаем информацию о webhook
		info, err := botAPI.GetWebhookInfo()
		if err != nil {
			logger.Error.Fatalf("Ошибка при получении информации о webhook: %v", err)
		}

		if info.LastErrorDate != 0 {
			logger.Warning.Printf("Последняя ошибка webhook от Telegram: %s (URL: %s)", info.LastErrorMessage, info.URL)
		} else if info.URL == publicWebhookURL {
			logger.Info.Printf("Webhook успешно установлен и подтвержден Telegram для URL: %s", info.URL)
		} else {
			logger.Warning.Printf("URL вебхука в Telegram (%s) не совпадает с настроенным (%s)", info.URL, publicWebhookURL)
		}

		// Настраиваем внутренний HTTP-сервер для приема обновлений от Nginx.
		// Nginx проксирует запросы с https://mb0.tech/webhook/BOT_TOKEN на http://localhost:PORT/BOT_TOKEN.
		// Поэтому ListenForWebhook должен слушать на "/"+botAPI.Token.
		internalWebhookPath := "/" + config.AppConfig.SecureWebhookToken
		updates := botAPI.ListenForWebhook(internalWebhookPath)
		logger.Info.Printf("Внутренний HTTP-сервер настроен на путь: %s", internalWebhookPath)

		// Добавляем обработчик для /superconnect
		http.HandleFunc("/superconnect", func(w http.ResponseWriter, r *http.Request) {
			// Проверяем, что метод запроса - POST
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Парсим форму для получения параметров
			if err := r.ParseForm(); err != nil {
				logger.Error.Printf("Ошибка при парсинге формы: %v", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			// Получаем параметры
			senderID := r.FormValue("sender_id")
			message := r.FormValue("message")
			accepterID := r.FormValue("accepter_id")
			token := r.FormValue("super_connect_token")

			// Проверяем токен
			if token != config.AppConfig.SuperConnectToken {
				logger.Error.Printf("Неверный токен: %s", token)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Проверяем наличие всех необходимых параметров
			if senderID == "" || message == "" || accepterID == "" {
				logger.Error.Printf("Отсутствуют обязательные параметры")
				http.Error(w, "Missing required parameters", http.StatusBadRequest)
				return
			}

			// Преобразуем accepterID в int64
			userID, err := strconv.ParseInt(accepterID, 10, 64)
			if err != nil {
				logger.Error.Printf("Ошибка при преобразовании accepter_id: %v", err)
				http.Error(w, "Invalid accepter_id", http.StatusBadRequest)
				return
			}

			// Получаем информацию о пользователе
			user, err := database.GetUserByID(userID)
			if err != nil {
				logger.Error.Printf("Пользователь не найден: %v", err)
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}

			// Формируем сообщение с ФИО пользователя
			fullMessage := fmt.Sprintf("📢 *Уведомление*\n\nОт: %s\n\n%s", user.FullName, message)

			// Отправляем сообщение пользователю
			msg := tgbotapi.NewMessage(userID, fullMessage)
			msg.ParseMode = "Markdown"
			_, err = botAPI.Send(msg)
			if err != nil {
				logger.Error.Printf("Ошибка при отправке сообщения: %v", err)
				http.Error(w, "Failed to send message", http.StatusInternalServerError)
				return
			}

			// Отправляем успешный ответ
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Message sent successfully"))
		})

		// Запускаем HTTP-сервер в отдельной горутине на внутреннем порту *port
		go func() {
			logger.Info.Printf("Запуск внутреннего webhook HTTP-сервера на порту %s", *port)
			// http.ListenAndServe будет слушать на всех интерфейсах на :*port
			// Путь internalWebhookPath обрабатывается через botAPI.ListenForWebhook
			err := http.ListenAndServe(":"+*port, nil)
			if err != nil {
				// Эта ошибка возникнет, если сервер не сможет запуститься (например, порт занят)
				// или если он неожиданно остановится.
				// Если остановка плановая (через sigChan), эта горутина просто завершится.
				// Для более чистого завершения HTTP-сервера при сигнале можно использовать http.Server с Shutdown().
				logger.Error.Fatalf("Ошибка при работе внутреннего webhook HTTP-сервера: %v", err)
			}
		}()

		// Обрабатываем обновления
		go func() {
			for update := range updates {
				if !isRunning {
					break
				}

				wg.Add(1)
				go func(upd tgbotapi.Update) {
					defer wg.Done()
					handleUpdate(botAPI, upd)
				}(update)
			}
			logger.Info.Println("Канал обновлений закрыт, прекращаем прием новых задач.")
		}()
	}

	// Начинаем обработку сообщений
	logger.Info.Println("Начинаем обработку сообщений")

	// Ожидаем сигнал завершения
	<-sigChan

	// Устанавливаем флаг завершения
	logger.Info.Println("Получен сигнал завершения, останавливаем обработку новых обновлений...")
	isRunning = false

	// Если использовался webhook, удаляем его при завершении
	if *webhookHost != "" {
		// _, err := botAPI.RemoveWebhook() // Старый способ
		_, err := botAPI.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}) // Новый способ удаления вебхука
		if err != nil {
			logger.Error.Printf("Ошибка при удалении webhook: %v", err)
		} else {
			logger.Info.Println("Webhook успешно удален")
		}
	}

	logger.Info.Println("Ожидание завершения активных обработчиков...")
	// Создаем канал для сигнализации о завершении wg.Wait()
	waitGroupDone := make(chan struct{})
	go func() {
		wg.Wait() // Ожидаем завершения всех горутин handleUpdate
		close(waitGroupDone)
	}()

	// Устанавливаем таймаут для ожидания
	select {
	case <-waitGroupDone:
		logger.Info.Println("Все обработчики успешно завершили работу.")
	case <-time.After(30 * time.Second): // Таймаут, например, 30 секунд
		logger.Error.Println("Тайм-аут ожидания завершения обработчиков. Некоторые задачи могли не завершиться.")
	}

	// Закрываем соединение с базой данных и завершаем программу
	logger.Info.Println("Закрываем соединения...")
	if database.DB != nil {
		err := database.DB.Close()
		if err != nil {
			logger.Error.Printf("Ошибка при закрытии соединения с БД: %v", err)
		} else {
			logger.Info.Println("Соединение с базой данных успешно закрыто.")
		}
	}
	logger.Info.Println("Бот завершает работу")
}

// handleUpdate обрабатывает обновления от Telegram API
func handleUpdate(botAPI *tgbotapi.BotAPI, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error.Printf("Восстановление после паники при обработке обновления: %v", r)
		}
	}()

	// Обрабатываем только сообщения
	if update.Message == nil {
		return
	}

	logger.Info.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

	// Проверяем, является ли сообщение командой
	if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "start":
			bot.HandleStart(botAPI, update.Message)
		case "help":
			// Добавляем обработку команды help
			helpText := "🤖 *Справка по использованию бота*\n\n" +
				"Этот бот предназначен для создания и управления тикетами поддержки.\n\n" +
				"*Основные команды:*\n" +
				"/start - Начать работу с ботом\n" +
				"/help - Показать эту справку\n" +
				"/ticket <ID> - Просмотр информации о тикете\n\n" +
				"*Основные функции:*\n" +
				"• Создание новых тикетов\n" +
				"• Просмотр активных тикетов\n" +
				"• Просмотр истории тикетов\n" +
				"• Обмен сообщениями с поддержкой\n" +
				"• Отправка фотографий в тикеты"

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
			msg.ParseMode = "Markdown"
			bot.SafeSendMessage(botAPI, msg)

			// Отправляем случайный совет
			bot.SendRandomTip(botAPI, update.Message.Chat.ID)
		case "ticket":
			// Обработка команды /ticket <ID>
			args := update.Message.CommandArguments()
			if args == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Пожалуйста, укажите ID тикета: /ticket <ID>")
				bot.SafeSendMessage(botAPI, msg)
				return
			}

			// Преобразуем ID тикета в число
			ticketID, err := strconv.Atoi(args)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Некорректный ID тикета. Используйте формат: /ticket <ID>")
				bot.SafeSendMessage(botAPI, msg)
				return
			}

			// Получаем информацию о тикете
			ticket, err := database.GetTicketByID(ticketID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении тикета %d: %v", ticketID, err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Тикет не найден или произошла ошибка при его получении.")
				bot.SafeSendMessage(botAPI, msg)
				return
			}

			// Проверяем, принадлежит ли тикет пользователю
			if ticket.UserID != update.Message.From.ID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ У вас нет доступа к этому тикету.")
				bot.SafeSendMessage(botAPI, msg)
				return
			}

			// Получаем сообщения тикета
			messages, err := database.GetTicketMessages(ticketID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении сообщений тикета %d: %v", ticketID, err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Ошибка при получении сообщений тикета.")
				bot.SafeSendMessage(botAPI, msg)
				return
			}

			// Форматируем даты
			createdDate := ticket.CreatedAt.Format("02.01.2006 15:04")
			closedDate := ""
			if ticket.Status == "закрыт" && ticket.ClosedAt.Valid {
				closedDate = fmt.Sprintf("\n🔒 Закрыт: %s", ticket.ClosedAt.Time.Format("02.01.2006 15:04"))
			}

			// Определяем эмодзи статуса
			statusEmoji := bot.GetStatusEmoji(ticket.Status)

			// Создаем сообщение с информацией о тикете
			ticketInfo := fmt.Sprintf(
				"🔖 *Тикет #%d*\n%s %s\n\n📝 Категория: %s\n📅 Создан: %s%s\n💬 Сообщений: %d\n\n*Описание:*\n%s",
				ticket.ID,
				statusEmoji,
				strings.ReplaceAll(ticket.Title, "*", "\\*"), // Экранируем звездочки
				ticket.Category,
				createdDate,
				closedDate,
				len(messages),
				strings.ReplaceAll(ticket.Description, "*", "\\*"), // Экранируем звездочки
			)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, ticketInfo)
			msg.ParseMode = "Markdown"
			bot.SafeSendMessage(botAPI, msg)

			// Отправляем историю сообщений
			if len(messages) > 0 {
				historyMsg := "📜 *История сообщений:*\n\n"
				for i, m := range messages {
					senderType := "👤 Вы"
					if m.SenderType == "admin" || m.SenderType == "support" {
						senderType = "👨‍💼 Поддержка"
					}
					msgTime := m.CreatedAt.Format("02.01.2006 15:04")
					// Экранируем специальные символы в сообщении
					messageText := strings.ReplaceAll(m.Message, "*", "\\*")
					messageText = strings.ReplaceAll(messageText, "_", "\\_")
					historyMsg += fmt.Sprintf("%d. %s (%s):\n%s\n\n", i+1, senderType, msgTime, messageText)
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, historyMsg)
				msg.ParseMode = "Markdown"
				bot.SafeSendMessage(botAPI, msg)
			}

			// Получаем и отправляем фотографии тикета
			photos, err := database.GetTicketPhotos(ticketID)
			if err != nil {
				logger.Error.Printf("Ошибка при получении фотографий тикета %d: %v", ticketID, err)
			} else if len(photos) > 0 {
				photoMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "📸 *Прикрепленные фотографии:*")
				photoMsg.ParseMode = "Markdown"
				bot.SafeSendMessage(botAPI, photoMsg)

				for i, photo := range photos {
					file, err := os.Open(photo.FilePath)
					if err != nil {
						logger.Error.Printf("Ошибка при открытии файла %s: %v", photo.FilePath, err)
						continue
					}
					defer file.Close()

					ext := filepath.Ext(photo.FilePath)
					if ext == "" {
						ext = ".jpg"
					}

					photoConfig := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FileReader{
						Name:   fmt.Sprintf("photo_%d%s", i+1, ext),
						Reader: file,
					})

					caption := fmt.Sprintf("📷 Фото #%d", i+1)
					if photo.SenderType == "user" {
						caption += " (от вас)"
					} else {
						caption += " (от поддержки)"
					}
					photoConfig.Caption = caption

					_, err = botAPI.Send(photoConfig)
					if err != nil {
						logger.Error.Printf("Ошибка при отправке фото %s: %v", photo.FilePath, err)
					}

					time.Sleep(100 * time.Millisecond) // Небольшая задержка между отправкой фото
				}
			}

			// Показываем клавиатуру для навигации
			keyboard := tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("⬅️ Назад к истории"),
				),
			)
			navMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "Используйте кнопку ниже для возврата к истории тикетов")
			navMsg.ReplyMarkup = keyboard
			bot.SafeSendMessage(botAPI, navMsg)
		default:
			// Неизвестные команды обрабатываем как обычные сообщения
			bot.HandleMessage(botAPI, update.Message)
		}
	} else {
		// Обычные сообщения
		bot.HandleMessage(botAPI, update.Message)
	}
}
