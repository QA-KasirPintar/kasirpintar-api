package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/url"
	"os"
	"strconv"
	"time"

	gomail "github.com/go-mail/mail/v2"
	"golang.org/x/crypto/bcrypt"
)

// SendResetEmail generates a reset token, stores its hash, and sends an email via Brevo (SMTP)
func SendResetEmail(email string) error {
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// Do not reveal whether email exists
		return nil
	}

	// Generate token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("failed generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	// Hash token
	tokenHashBytes, err := bcrypt.GenerateFromPassword([]byte(token), 10)
	if err != nil {
		return fmt.Errorf("failed hash token: %w", err)
	}

	reset := models.PasswordReset{
		UserID:    user.ID,
		TokenHash: string(tokenHashBytes),
		ExpiresAt: time.Now().Add(time.Hour * 1),
	}
	if err := config.DB.Create(&reset).Error; err != nil {
		return fmt.Errorf("failed save reset: %w", err)
	}

	// Build frontend reset link
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	u, _ := url.Parse(frontendURL)
	u.Path = "/reset-password"
	q := u.Query()
	q.Set("token", token)
	q.Set("email", user.Email)
	u.RawQuery = q.Encode()
	resetLink := u.String()

	// Compose HTML email
	subject := "Reset Password KasirPintar"
	htmlBody := fmt.Sprintf(`<html><body><p>Silakan klik link berikut untuk mereset password Anda:</p><p><a href="%s">Reset Password</a></p><p>Atau salin link berikut:</p><p>%s</p><p>Link berlaku 1 jam.</p></body></html>`, resetLink, resetLink)

	// Send via Brevo SMTP using go-mail
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "smtp-relay.brevo.com"
	}
	portStr := os.Getenv("SMTP_PORT")
	if portStr == "" {
		portStr = "587"
	}
	port, _ := strconv.Atoi(portStr)
	userSMTP := os.Getenv("SMTP_USER")
	passSMTP := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("FROM_EMAIL")
	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		fromName = "KasirPintar"
	}

	// If SMTP not configured, print to stdout for dev
	if userSMTP == "" || passSMTP == "" || fromEmail == "" {
		fmt.Printf("[mail dev] To: %s\nSubject: %s\n\n%s\n", email, subject, resetLink)
		return nil
	}

	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(fromEmail, fromName))
	m.SetHeader("To", email)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	d := gomail.NewDialer(host, port, userSMTP, passSMTP)
	// Brevo supports TLS; leave default settings
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send email failed: %w", err)
	}

	return nil
}

// VerifyAndReset verifies token and resets password
func VerifyAndReset(email string, token string, newPassword string) error {
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return fmt.Errorf("invalid token or expired")
	}

	var reset models.PasswordReset
	if err := config.DB.Where("user_id = ?", user.ID).Order("created_at DESC").First(&reset).Error; err != nil {
		return fmt.Errorf("invalid token or expired")
	}
	if time.Now().After(reset.ExpiresAt) {
		return fmt.Errorf("token expired")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(reset.TokenHash), []byte(token)); err != nil {
		return fmt.Errorf("invalid token or expired")
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return fmt.Errorf("failed hash new password: %w", err)
	}
	if err := config.DB.Model(&user).Update("password", string(newHash)).Error; err != nil {
		return fmt.Errorf("failed update password: %w", err)
	}

	// Delete reset tokens
	config.DB.Where("user_id = ?", user.ID).Delete(&models.PasswordReset{})
	return nil
}
