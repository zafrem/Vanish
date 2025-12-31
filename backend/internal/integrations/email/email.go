package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
)

// Config holds SMTP configuration
type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	FromName     string
}

// Client represents an email client
type Client struct {
	config *Config
}

// NewClient creates a new email client
func NewClient(config *Config) *Client {
	return &Client{config: config}
}

// SendSecretNotification sends an email notification about a new secret
func (c *Client) SendSecretNotification(recipientEmail, recipientName, senderName, secretURL string) error {
	subject := fmt.Sprintf("🔒 Secure Message from %s", senderName)

	htmlBody, err := c.renderSecretNotificationHTML(recipientName, senderName, secretURL)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	plainBody := c.renderSecretNotificationPlain(recipientName, senderName, secretURL)

	return c.sendEmail(recipientEmail, subject, htmlBody, plainBody)
}

func (c *Client) sendEmail(to, subject, htmlBody, plainBody string) error {
	from := fmt.Sprintf("%s <%s>", c.config.FromName, c.config.FromAddress)

	// Build email message
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "multipart/alternative; boundary=\"boundary123\""

	var msg bytes.Buffer
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	msg.WriteString("\r\n--boundary123\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	msg.WriteString(plainBody)

	msg.WriteString("\r\n--boundary123\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	msg.WriteString(htmlBody)

	msg.WriteString("\r\n--boundary123--")

	// Setup authentication
	auth := smtp.PlainAuth("", c.config.SMTPUser, c.config.SMTPPassword, c.config.SMTPHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)
	err := smtp.SendMail(addr, auth, c.config.FromAddress, []string{to}, msg.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (c *Client) renderSecretNotificationHTML(recipientName, senderName, secretURL string) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #ef4444, #f97316); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: linear-gradient(135deg, #ef4444, #f97316); color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; margin: 20px 0; }
        .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 15px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #6b7280; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔒 Secure Message</h1>
        </div>
        <div class="content">
            <p>Hi {{.RecipientName}},</p>
            <p><strong>{{.SenderName}}</strong> has sent you a secure, ephemeral message via Vanish.</p>

            <div style="text-align: center;">
                <a href="{{.SecretURL}}" class="button">View Secret Message</a>
            </div>

            <div class="warning">
                <p><strong>⚠️ Important:</strong></p>
                <ul>
                    <li>This message can only be viewed <strong>once</strong></li>
                    <li>It will be <strong>permanently destroyed</strong> after you read it</li>
                    <li>The content will be copied directly to your clipboard</li>
                    <li>Do not share this link with anyone else</li>
                </ul>
            </div>

            <p style="color: #6b7280; font-size: 14px;">
                This is an automated message from Vanish - Secure Ephemeral Messaging Platform
            </p>
        </div>
        <div class="footer">
            <p>If you did not expect this message, please contact your security team.</p>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		RecipientName string
		SenderName    string
		SecretURL     string
	}{
		RecipientName: recipientName,
		SenderName:    senderName,
		SecretURL:     secretURL,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *Client) renderSecretNotificationPlain(recipientName, senderName, secretURL string) string {
	return fmt.Sprintf(`
Hi %s,

%s has sent you a secure, ephemeral message via Vanish.

Click here to view: %s

IMPORTANT:
- This message can only be viewed ONCE
- It will be permanently destroyed after you read it
- The content will be copied directly to your clipboard
- Do not share this link with anyone else

---
This is an automated message from Vanish - Secure Ephemeral Messaging Platform

If you did not expect this message, please contact your security team.
`, recipientName, senderName, secretURL)
}
