package bot

import (
	fmt "fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Создаем клавиатуру с кнопкой для отправки контакта
func GetContactKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonContact("Поделиться контактом"),
		),
	)
	keyboard.OneTimeKeyboard = true
	return keyboard
}

// Эта функция больше не будет использоваться в текущем коде,
// но её можно оставить на случай будущих изменений
func GetLocationKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("Поделиться местоположением"),
		),
	)
	keyboard.OneTimeKeyboard = true
	return keyboard
}

// Создаем главное меню бота с современным дизайном
func GetMainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🎯 Активные тикеты"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📚 История тикетов"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✨ Создать тикет"),
		),
	)
	keyboard.ResizeKeyboard = true
	return keyboard
}

// Создаем клавиатуру подтверждения с современными эмодзи
func GetConfirmKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("✅ Да"),
			tgbotapi.NewKeyboardButton("❌ Нет"),
		),
	)
	keyboard.OneTimeKeyboard = true
	return keyboard
}

// Создаем клавиатуру категорий тикетов с современными эмодзи
func GetCategoryKeyboard() tgbotapi.ReplyKeyboardMarkup {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💭 Вопрос"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚨 Важно,Срочно"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("💰 Финансы"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("❌ Отмена"),
		),
	)
	keyboard.OneTimeKeyboard = true
	return keyboard
}

// Создаем inline клавиатуру для тикета с современными эмодзи
func GetTicketInlineKeyboard(ticketID int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📷 Фото", fmt.Sprintf("photos_%d", ticketID)),
			tgbotapi.NewInlineKeyboardButtonData("📈 Статус", fmt.Sprintf("status_%d", ticketID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 Ответить", fmt.Sprintf("reply_%d", ticketID)),
			tgbotapi.NewInlineKeyboardButtonData("🔒 Закрыть", fmt.Sprintf("close_%d", ticketID)),
		),
	)
}
