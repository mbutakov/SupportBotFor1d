package logger

import (
	"io"
	"log"
	"os"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

// InitLogger инициализирует логеры с записью в файл и консоль
func InitLogger(logFilePath string) error {
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Мультиплексируем вывод в файл и консоль
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Инициализируем логеры
	Info = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(multiWriter, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}
