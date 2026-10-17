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

const (
	TaskPasswordReset = "email:password_reset"
	TaskNotice        = "email:notice"
)

type noticePayload struct {
	To      string `json:"to"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type passwordResetPayload struct {
	To   string `json:"to"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// SMTPMailer mengirim email lewat SMTP (Mailpit di lokal).
type SMTPMailer struct{ cfg config.Config }

func NewSMTPMailer(cfg config.Config) *SMTPMailer { return &SMTPMailer{cfg: cfg} }

func (m *SMTPMailer) SendPasswordReset(to, name, link string) error {
	body := "Halo " + name + ",\n\nKami menerima permintaan untuk mengatur ulang password akun SiKerja Anda.\n" +
		"Buka tautan berikut (berlaku 1 jam, sekali pakai):\n\n" + link + "\n\n" +
		"Jika Anda tidak memintanya, abaikan email ini.\n"
	return m.send(to, "Atur ulang password SiKerja", body)
}

// SendNotice mengirim email notifikasi biasa.
func (m *SMTPMailer) SendNotice(to, _, subject, body string) error { return m.send(to, subject, body) }

func (m *SMTPMailer) send(to, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, body)

	opts := []gomail.Option{gomail.WithPort(m.cfg.SMTPPort), gomail.WithTLSPolicy(gomail.NoTLS)}
	if m.cfg.SMTPUser != "" {
		// kredensial tidak boleh lewat koneksi polos: di production TLS wajib (STARTTLS di 587, TLS langsung di 465)
		policy := gomail.TLSOpportunistic
		if m.cfg.IsProd() {
			policy = gomail.TLSMandatory
		}
		opts = append(opts, gomail.WithTLSPolicy(policy))
		if m.cfg.SMTPPort == 465 {
			opts = append(opts, gomail.WithSSL())
		}
		opts = append(opts,
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

// SendNotice memasukkan email notifikasi ke antrian.
func (q *QueueMailer) SendNotice(to, name, subject, body string) error {
	b, _ := json.Marshal(noticePayload{To: to, Name: name, Subject: subject, Body: body})
	_, err := q.client.Enqueue(asynq.NewTask(TaskNotice, b), asynq.MaxRetry(5), asynq.Queue("default"))
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
	mux.HandleFunc(TaskNotice, func(_ context.Context, t *asynq.Task) error {
		var p noticePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("payload: %w: %w", err, asynq.SkipRetry)
		}
		return m.SendNotice(p.To, p.Name, p.Subject, p.Body)
	})
	return mux
}
