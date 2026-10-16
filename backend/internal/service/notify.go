package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// Jenis notifikasi (kolom notifications.type).
const (
	EvLeaveSubmitted = "leave_submitted"
	EvLeaveProgress  = "leave_progress"
	EvLeaveApproved  = "leave_approved"
	EvLeaveRejected  = "leave_rejected"
	EvLeaveCancelled = "leave_cancelled"
	EvPeriodOpened   = "period_opened"
	EvDeadline       = "deadline_reminder"
)

// NotifySettings adalah saklar notifikasi di H-16: nama pengaturan → jenis notifikasi yang dikendalikannya.
var NotifySettings = []struct {
	Key, Label, Hint string
	Events           []string
}{
	{"notify.leave_submitted", "Pengajuan izin/cuti baru", "ke penyetuju berikutnya", []string{EvLeaveSubmitted, EvLeaveCancelled}},
	{"notify.leave_result", "Hasil persetujuan", "ke karyawan yang mengajukan", []string{EvLeaveProgress, EvLeaveApproved, EvLeaveRejected}},
	{"notify.period_opened", "Periode penilaian dibuka", "ke semua karyawan", []string{EvPeriodOpened}},
	{"notify.deadline_reminder", "Pengingat tenggat penilaian", "3 hari dan 1 hari sebelum tenggat", []string{EvDeadline}},
}

// Notice adalah satu pesan untuk satu atau banyak penerima.
type Notice struct {
	Type    string
	Title   string // subjek email
	Message string
	Link    string // path di aplikasi, mis. /persetujuan
	Data    map[string]any
}

// Recipient adalah tujuan akhir sebuah kanal.
type Recipient struct {
	UserID uint64
	Name   string
	Email  string
}

// Channel adalah kanal pengiriman tambahan di luar notifikasi dalam aplikasi (email sekarang; WhatsApp nanti).
// Untuk menambah kanal baru cukup implementasikan interface ini dan daftarkan di NewNotifyService.
type Channel interface {
	Name() string
	Deliver(ctx context.Context, to Recipient, n Notice) error
}

// Notifier dipakai service lain. Pengiriman bersifat best-effort: kegagalan dicatat, tidak menggagalkan
// operasi bisnis yang memicunya.
type Notifier interface {
	Notify(ctx context.Context, n Notice, to ...model.Employee)
}

// NoopNotifier tidak melakukan apa-apa (default untuk service yang notifikasinya tidak dibutuhkan).
type NoopNotifier struct{}

func (NoopNotifier) Notify(context.Context, Notice, ...model.Employee) {}

// EmailSender mengirim email biasa (lewat antrian di API, langsung di worker).
type EmailSender interface {
	SendNotice(toEmail, name, subject, body string) error
}

// Mailer adalah semua kemampuan email yang dibutuhkan aplikasi.
type Mailer interface {
	PasswordResetMailer
	EmailSender
}

// EmailChannel mengirim notifikasi lewat email.
type EmailChannel struct {
	Sender  EmailSender
	BaseURL string
}

func (EmailChannel) Name() string { return "email" }

func (c EmailChannel) Deliver(_ context.Context, to Recipient, n Notice) error {
	if to.Email == "" {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Halo %s,\n\n%s\n", to.Name, n.Message)
	if n.Link != "" {
		fmt.Fprintf(&b, "\nBuka SiKerja: %s%s\n", strings.TrimRight(c.BaseURL, "/"), n.Link)
	}
	b.WriteString("\nSalam,\nSiKerja\n")
	return c.Sender.SendNotice(to.Email, to.Name, n.Title, b.String())
}

// NotifyService menyimpan notifikasi dalam aplikasi lalu meneruskannya ke kanal tambahan.
type NotifyService struct {
	store    repository.Store
	channels []Channel
	log      zerolog.Logger
}

func NewNotifyService(store repository.Store, log zerolog.Logger, channels ...Channel) *NotifyService {
	return &NotifyService{store: store, channels: channels, log: log}
}

// eventEnabled: jenis notifikasi aktif kecuali dimatikan HR di pengaturan.
func eventEnabled(settings map[string]string, event string) bool {
	for _, s := range NotifySettings {
		for _, e := range s.Events {
			if e == event {
				return settings[s.Key] != "false"
			}
		}
	}
	return true
}

func (s *NotifyService) Notify(ctx context.Context, n Notice, to ...model.Employee) {
	if len(to) == 0 {
		return
	}
	r := s.store.R()
	settings, err := r.Settings.All()
	if err != nil {
		s.log.Error().Err(err).Msg("notifikasi: baca pengaturan")
		return
	}
	if !eventEnabled(settings, n.Type) {
		return
	}
	seen := map[uint64]bool{}
	for _, e := range to {
		if seen[e.UserID] {
			continue
		}
		seen[e.UserID] = true
		data := map[string]any{"title": n.Title, "message": n.Message, "link": n.Link}
		for k, v := range n.Data {
			data[k] = v
		}
		raw, _ := json.Marshal(data)
		if err := r.Notifications.Create(&model.Notification{UserID: e.UserID, Type: n.Type, Data: string(raw)}); err != nil {
			s.log.Error().Err(err).Uint64("user", e.UserID).Msg("notifikasi: simpan")
			continue
		}
		rec := Recipient{UserID: e.UserID, Name: e.Name}
		if e.User != nil {
			rec.Email = e.User.Email
		} else if u, err := r.Users.ByID(e.UserID); err == nil {
			rec.Email = u.Email
		}
		for _, ch := range s.channels {
			if err := ch.Deliver(ctx, rec, n); err != nil {
				s.log.Error().Err(err).Str("kanal", ch.Name()).Uint64("user", e.UserID).Msg("notifikasi: kirim")
			}
		}
	}
}

// ---------- pengaturan notifikasi (H-16) ----------

type NotifySetting struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Hint    string `json:"hint"`
	Enabled bool   `json:"enabled"`
}

// Settings mengembalikan saklar notifikasi (default aktif).
func (s *NotifyService) Settings(a Actor) ([]NotifySetting, error) {
	if !a.IsHR() {
		return nil, errForbiddenData
	}
	m, err := s.store.R().Settings.All()
	if err != nil {
		return nil, err
	}
	out := make([]NotifySetting, 0, len(NotifySettings))
	for _, d := range NotifySettings {
		out = append(out, NotifySetting{Key: d.Key, Label: d.Label, Hint: d.Hint, Enabled: m[d.Key] != "false"})
	}
	return out, nil
}

func (s *NotifyService) SetSetting(a Actor, key string, enabled bool) error {
	if !a.IsHR() {
		return errForbiddenData
	}
	known := false
	for _, d := range NotifySettings {
		known = known || d.Key == key
	}
	if !known {
		return errUnknownSetting
	}
	return s.store.Tx(func(r repository.Repos) error {
		old, _ := r.Settings.All()
		if err := r.Settings.Set(key, fmt.Sprintf("%t", enabled)); err != nil {
			return err
		}
		return logActivity(r, a, AuditSettingChange, "setting", 0,
			map[string]any{key: old[key] != "false"}, map[string]any{key: enabled})
	})
}
