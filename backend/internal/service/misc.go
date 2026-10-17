package service

import (
	"time"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

// NotificationService: daftar notifikasi in-app (lonceng). Pembuatan otomatis menyusul di Rilis 2.
type NotificationService struct{ store repository.Store }

func NewNotificationService(store repository.Store) *NotificationService {
	return &NotificationService{store: store}
}

type NotificationList struct {
	Items  []model.Notification `json:"items"`
	Unread int64                `json:"unread"`
}

func (s *NotificationService) List(a Actor) (*NotificationList, error) {
	r := s.store.R()
	items, err := r.Notifications.ListByUser(a.UserID, 30)
	if err != nil {
		return nil, err
	}
	n, err := r.Notifications.UnreadCount(a.UserID)
	if err != nil {
		return nil, err
	}
	return &NotificationList{Items: items, Unread: n}, nil
}

func (s *NotificationService) MarkRead(a Actor, id uint64) error {
	return s.store.R().Notifications.MarkRead(a.UserID, id, time.Now())
}

func (s *NotificationService) MarkAllRead(a Actor) error {
	return s.store.R().Notifications.MarkAllRead(a.UserID, time.Now())
}

// AuditService: HR dapat menelusuri activity_log.
type AuditService struct{ store repository.Store }

func NewAuditService(store repository.Store) *AuditService { return &AuditService{store: store} }

func (s *AuditService) List(a Actor, f repository.AuditFilter) ([]model.ActivityLog, int64, error) {
	if !a.IsHR() {
		return nil, 0, apperr.Forbidden("Hanya HR yang dapat melihat audit log")
	}
	return s.store.R().Audit.List(f)
}

// Aspects: daftar aspek perilaku (dibaca semua pengguna login).
func (s AppraisalService) Aspects() ([]model.BehaviorAspect, error) {
	return s.store.R().Aspects.List()
}
