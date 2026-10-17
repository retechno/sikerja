// Package mail mengirim email: langsung via SMTP (worker) atau lewat antrian asynq (API).
package mail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	gomail "github.com/wneessen/go-mail"

	"github.com/retechno/sikerja/backend/internal/config"
)

const TaskPasswordReset = "email:password_reset"

type passwordResetPayload struct {
	To   string `json:"to"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// SMTPMailer mengirim email lewat SMTP (Mailpit di lokal).
type SMTPMailer struct{ cfg config.Config }

func NewSMTPMailer(cfg config.Config) *SMTPMailer { return &SMTPMailer{cfg: cfg} }

func (m *SMTPMailer) SendPasswordReset(to, name, link string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("to: %w", err)
	}
	msg.Subject("Atur ulang password SiKerja")
	msg.SetBodyString(gomail.TypeTextPlain, fmt.Sprintf(
		"Halo %s,\n\nKami menerima permintaan untuk mengatur ulang password akun SiKerja Anda.\n"+
			"Buka tautan berikut (berlaku 1 jam, sekali pakai):\n\n%s\n\n"+
			"Jika Anda tidak memintanya, abaikan email ini.\n", name, link))

	opts := []gomail.Option{gomail.WithPort(m.cfg.SMTPPort), gomail.WithTLSPolicy(gomail.NoTLS)}
	if m.cfg.SMTPUser != "" {
		opts = append(opts,
			gomail.WithTLSPolicy(gomail.TLSOpportunistic),
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(m.cfg.SMTPUser), gomail.WithPassword(m.cfg.SMTPPass))
	}
	c, err := gomail.NewClient(m.cfg.SMTPHost, opts...)
	if err != nil {
		return err
	}
	return c.DialAndSend(msg)
}

// QueueMailer memasukkan email ke antrian asynq; worker yang mengirimnya.
type QueueMailer struct{ client *asynq.Client }

func NewQueueMailer(redisAddr, redisPass string) *QueueMailer {
	return &QueueMailer{client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr, Password: redisPass})}
}

func (q *QueueMailer) Close() error { return q.client.Close() }

func (q *QueueMailer) SendPasswordReset(to, name, link string) error {
	b, _ := json.Marshal(passwordResetPayload{To: to, Name: name, Link: link})
	_, err := q.client.Enqueue(asynq.NewTask(TaskPasswordReset, b), asynq.MaxRetry(5), asynq.Queue("default"))
	return err
}

// Handler mendaftarkan handler tugas email untuk worker.
func Handler(m *SMTPMailer) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskPasswordReset, func(_ context.Context, t *asynq.Task) error {
		var p passwordResetPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("payload: %w: %w", err, asynq.SkipRetry)
		}
		return m.SendPasswordReset(p.To, p.Name, p.Link)
	})
	return mux
}
