// Package mailer sends verification codes via email using the Resend API.
package mailer

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/resend/resend-go/v4"

	"sspu-verifier/internal/config"
)

const sendTimeout = 30 * time.Second

type Mailer struct {
	client *resend.Client
	cfg    config.Email
}

func New(cfg config.Email) *Mailer {
	return &Mailer{client: resend.NewClient(cfg.APIKey), cfg: cfg}
}

func (m *Mailer) SendCode(to, subject, code string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    m.cfg.From,
		To:      []string{to},
		Subject: subject,
		Text:    m.buildText(code, ttl),
		Html:    m.buildHTML(code, ttl),
	})
	if err != nil {
		return fmt.Errorf("sending email via Resend: %w", err)
	}
	return nil
}

func (m *Mailer) buildText(code string, ttl time.Duration) string {
	return strings.Join([]string{
		"Hello,",
		"",
		"Your verification code for Discord is:",
		"",
		"    " + code,
		"",
		fmt.Sprintf("The code is valid for %d minutes. If you did not request this, please ignore this email.", int(ttl.Minutes())),
		"",
		senderName(m.cfg.From),
	}, "\r\n")
}

var htmlTpl = template.Must(template.New("code").Parse(`<!doctype html>
<html lang="en">
<body style="margin:0;padding:0;background-color:#f4f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="max-width:480px;width:100%;background-color:#ffffff;border-radius:12px;">
<tr><td colspan="3" style="height:32px;"></td></tr>
<tr><td style="width:32px;"></td><td style="font-size:20px;font-weight:700;color:#111827;">Discord Verification</td><td style="width:32px;"></td></tr>
<tr><td colspan="3" style="height:8px;"></td></tr>
<tr><td style="width:32px;"></td><td style="font-size:15px;line-height:22px;color:#374151;">Hello,<br>Your verification code for Discord is:</td><td style="width:32px;"></td></tr>
<tr><td style="width:32px;"></td><td align="center" style="padding:24px 0;"><div style="display:inline-block;background-color:#eef2ff;border:1px solid #c7d2fe;border-radius:10px;padding:14px 28px;font-family:'SF Mono',Consolas,Menlo,monospace;font-size:34px;font-weight:700;letter-spacing:10px;text-indent:10px;color:#1d4ed8;">{{.Code}}</div></td><td style="width:32px;"></td></tr>
<tr><td style="width:32px;"></td><td style="font-size:13px;line-height:20px;color:#6b7280;">The code is valid for {{.Minutes}} min. If you did not request this, please ignore this email.</td><td style="width:32px;"></td></tr>
<tr><td colspan="3" style="height:24px;"></td></tr>
<tr><td style="width:32px;"></td><td style="font-size:13px;color:#9ca3af;">{{.Sender}}</td><td style="width:32px;"></td></tr>
<tr><td colspan="3" style="height:32px;"></td></tr>
</table>
</td></tr></table>
</body>
</html>`))

func (m *Mailer) buildHTML(code string, ttl time.Duration) string {
	var sb strings.Builder
	err := htmlTpl.Execute(&sb, map[string]any{
		"Code":    code,
		"Minutes": int(ttl.Minutes()),
		"Sender":  senderName(m.cfg.From),
	})
	if err != nil {
		return ""
	}
	return sb.String()
}

func senderName(from string) string {
	if open := strings.Index(from, "<"); open > 0 {
		return strings.TrimSpace(from[:open])
	}
	return "Discord bot"
}
