package config

import (
	"encoding/json"
	"os"
)

// Config содержит все настройки приложения
type Config struct {
	TelegramToken string `json:"telegram_token"`
	Database      struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
		SSLMode  string `json:"sslmode"`
	} `json:"database"`
	LogFile            string `json:"log_file"`
	SecureWebhookToken string `json:"secure_webhook_token"`
    SuperConnectToken string `json:"super_connect_token"`
}

// Глобальная переменная конфигурации
var AppConfig Config

// LoadConfig загружает конфигурацию из файла
func LoadConfig(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&AppConfig)
	if err != nil {
		return err
	}

	return nil
}
