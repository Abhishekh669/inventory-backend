package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PostgressURL  string
	FrontendURL   string
	JWTToken      string
	AdminEmail    string
	AdminPassword string
	AdminPhone    string
	// SMTPEmail        string
	// SMTPPassword     string
	// SMTPHost         string
	// SMTPPort         string
	// BrevoEmail       string
	// BrevoPassword    string
	// BrevoApiKey      string
	// BrevoSenderEmail string
	// BrevoSenderName  string
}

var AppConfig *Config

func InitConfig() error {
	if err := godotenv.Load(); err != nil {
		log.Printf("Waring : Couldnot load .env files : %v", err)
		return fmt.Errorf("couldn't load .env files : %v", err)
	}

	AppConfig = &Config{
		PostgressURL:  os.Getenv("DATABASE_URL"),
		FrontendURL:   os.Getenv("PUBLIC_FRONTEND_URL"),
		JWTToken:      os.Getenv("JWT_TOKEN"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		AdminPhone:    os.Getenv("ADMIN_PHONE"),
		// SMTPEmail:       os.Getenv("SMTP_EMAIL"),
		// SMTPPassword:    os.Getenv("SMTP_PASSWORD"),
		// SMTPHost:        os.Getenv("SMTP_HOST"),
		// SMTPPort:        os.Getenv("SMTP_PORT"),
		// BrevoEmail:      os.Getenv("BREVO_FROM_EMAIL"),
		// BrevoPassword:   os.Getenv("BREVO_SMTP_PASSWORD"),
		// BrevoSenderName: os.Getenv("BREVO_FROM_NAME"),
		// BrevoApiKey:     os.Getenv("BREVO_API_KEY"),
	}

	return nil
}
