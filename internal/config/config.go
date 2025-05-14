package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for our application
type Config struct {
	Port                      string
	Origin                    string
	Environment               string
	JWTSecret                 string
	JWTRefreshSecret          string
	JWTPasswordReset          string
	CookieSecret              string
	Database                  DatabaseConfig
	Mailer                    MailerConfig
	Google                    GoogleOAuthConfig
	JWTExpirationMinutes      int
	JWTRefreshExpirationHours int
	PasswordResetTokenExpiry  int
	VerificationTokenExpiry   int
	AppURL                    string
}

// DatabaseConfig holds database connection details
type DatabaseConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Name     string
	DSN      string
}

// MailerConfig holds email service configuration
type MailerConfig struct {
	Transport   string
	DefaultFrom string
}

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load database configuration
	dbConfig := DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "3306"),
		Username: getEnv("DB_USERNAME", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Name:     getEnv("DB_NAME", "medi"),
	}

	// Build DSN (Data Source Name) for MySQL connection
	dbConfig.DSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbConfig.Username, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name)

	// Load mailer configuration
	mailerConfig := MailerConfig{
		Transport:   getEnv("MAILER_TRANSPORT", ""),
		DefaultFrom: getEnv("MAILER_DEFAULT_FROM", ""),
	}

	// Load Google OAuth configuration
	googleConfig := GoogleOAuthConfig{
		ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		CallbackURL:  getEnv("GOOGLE_CALLBACK_URL", ""),
	}

	jwtExpMinutes, err := strconv.Atoi(getEnv("JWT_EXPIRATION_MINUTES", "15"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRATION_MINUTES: %w", err)
	}

	jwtRefreshExpHours, err := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRATION_HOURS", "168")) // 7 days
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRATION_HOURS: %w", err)
	}

	passwordResetTokenExpiry, err := strconv.Atoi(getEnv("PASSWORD_RESET_TOKEN_EXPIRY_MINUTES", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid PASSWORD_RESET_TOKEN_EXPIRY_MINUTES: %w", err)
	}

	verificationTokenExpiry, err := strconv.Atoi(getEnv("VERIFICATION_TOKEN_EXPIRY_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("invalid VERIFICATION_TOKEN_EXPIRY_HOURS: %w", err)
	}

	// Return complete configuration
	return &Config{
		Port:                      getEnv("PORT", "3001"),
		Origin:                    getEnv("ORIGIN", "http://localhost:4200"),
		Environment:               getEnv("NODE_ENV", "development"),
		JWTSecret:                 getEnv("JWT_SECRET", "default_jwt_secret"),
		JWTRefreshSecret:          getEnv("JWT_REFRESH_SECRET", "default_refresh_secret"),
		JWTPasswordReset:          getEnv("JWT_PASSWORD_SECRET", "default_password_reset_secret"),
		CookieSecret:              getEnv("COOKIE_SECRET", "default_cookie_secret"),
		Database:                  dbConfig,
		Mailer:                    mailerConfig,
		Google:                    googleConfig,
		JWTExpirationMinutes:      jwtExpMinutes,
		JWTRefreshExpirationHours: jwtRefreshExpHours,
		PasswordResetTokenExpiry:  passwordResetTokenExpiry,
		VerificationTokenExpiry:   verificationTokenExpiry,
		AppURL:                    getEnv("APP_URL", "http://localhost:3001"),
	}, nil
}

// Helper function to get environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
