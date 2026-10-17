package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/storage"
)

// Aksi audit izin & cuti.
const (
	AuditLeaveSubmit    = "leave.submit"
	AuditLeaveApprove   = "leave.approve"
	AuditLeaveReject    = "leave.reject"
	AuditLeaveCancel    = "leave.cancel"
	AuditLeaveTypeSave  = "leave_type.change"
	AuditHolidayChange  = "holiday.change"
	AuditSettingChange  = "setting.change"
	maxBackdateDays     = 7 // pengajuan boleh diajukan mundur paling jauh 7 hari
	maxLeaveReasonRunes = 1000
)

var (
	errUnknownSetting = apperr.BadRequest("Pengaturan tidak dikenal")
	pendingStatuses   = []string{model.LeavePendingManager, model.LeavePendingHR}
	approvedStatuses  = []string{model.LeaveApproved}
)

// LeaveService: pengajuan, persetujuan bertingkat, pembatalan, dan saldo izin/cuti.
type LeaveService struct {
	store    repository.Store
	files    storage.Store
	wd       *WorkingDayService
	notifier Notifier
	now      func() time.Time
}

func NewLeaveService(store repository.Store, files storage.Store, wd *WorkingDayService, notifier Notifier) *LeaveService {
	if notifier == nil {
		notifier = NoopNotifier{}
	}
	return &LeaveService{store: store, files: files, wd: wd, notifier: notifier, now: time.Now}
}

// SetClock mengganti jam (untuk test).
func (s *LeaveService) SetClock(f func() time.Time) { s.now = f }

func (s *LeaveService) today() time.Time { return DateOnly(s.now()) }

// ---------- tampilan ----------

type FlowStep struct {
	Level int      `json:"level"`
	Role  string   `json:"role"`
	Names []string `json:"names"`
}

type TimelineStep struct {
	Title string     `json:"title"`
	Actor string     `json:"actor"`
	Note  string     `json:"note,omitempty"`
	At    *time.Time `json:"at"`
	State string     `json:"state"` // done | current | todo | rejected | cancelled
}

type BalanceInfo struct {
	Quota     decimal.Decimal `json:"quota"`
	Used      decimal.Decimal `json:"used"`
	Pending   decimal.Decimal `json:"pending"` // menunggu, di luar pengajuan ini
	Available decimal.Decimal `json:"available"`
	After     decimal.Decimal `json:"after"` // sisa bila pengajuan ini disetujui
}

type Conflict struct {
	EmployeeName string `json:"employee_name"`
	TypeName     string `json:"type_name"`
	Range        string `json:"range"`
	Status       string `json:"status"`
}

// LeaveView adalah pengajuan beserta data turunan untuk UI.
type LeaveView struct {
	model.LeaveRequest
	StatusLabel string `json:"status_label"`
	CanCancel   bool   `json:"can_cancel"`
	CancelUntil string `json:"cancel_until,omitempty"`
	CanAct      bool   `json:"can_act"`
	IsLastStage bool   `json:"is_last_stage"` // persetujuan pada tahap ini menyelesaikan pengajuan
	HasFile     bool   `json:"has_attachment"`

	Timeline  []TimelineStep `json:"timeline,omitempty"`
	Skipped   []DayInfo      `json:"skipped,omitempty"`
	Balance   *BalanceInfo   `json:"balance,omitempty"`
	Conflicts []Conflict     `json:"conflicts,omitempty"`
	Flow      []FlowStep     `json:"flow,omitempty"`
}

func (s *LeaveService) canCancel(a Actor, l *model.LeaveRequest) bool {
	return l.EmployeeID == a.EmployeeID &&
		(model.LeavePending(l.Status) || l.Status == model.LeaveApproved) &&
		s.today().Before(DateOnly(l.StartDate))
}

func (s *LeaveService) baseView(a Actor, l *model.LeaveRequest) LeaveView {
	v := LeaveView{LeaveRequest: *l, StatusLabel: leaveStatusLabel[l.Status], HasFile: l.AttachmentPath != ""}
	v.CanCancel = s.canCancel(a, l)
	if v.CanCancel && l.EmployeeID == a.EmployeeID {
		v.CancelUntil = DateOnly(l.StartDate).AddDate(0, 0, -1).Format("2006-01-02")
	}
	v.CanAct = canActOn(a, l)
	if _, idx, ok := currentStage(l); ok && l.Employee != nil {
		v.IsLastStage = idx == len(stagesFor(l.Employee, l.HalfDay))-1
	}
	return v
}

// approverNames: nama penyetuju tahap st untuk pengaju e.
func approverNames(r repository.Repos, e *model.Employee, st stage) ([]string, error) {
	list, err := approversOf(r, e, st)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list))
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names, nil
}

// approversOf: orang yang berhak pada tahap st (untuk notifikasi dan pratinjau alur).
func approversOf(r repository.Repos, e *model.Employee, st stage) ([]model.Employee, error) {
	var out []model.Employee
	switch st.Kind {
	case kindDirector:
		ds, err := r.Employees.ByRole(model.RoleDirector)
		if err != nil {
			return nil, err
		}
		out = ds
	case kindHR:
		hs, err := r.Employees.ByRole(model.RoleHR)
		if err != nil {
			return nil, err
		}
		out = hs
	default:
		if e.ManagerID != nil {
			m, err := r.Employees.ByID(*e.ManagerID)
			if err == nil && m.Status == model.StatusActive {
				out = append(out, *m)
			}
		}
		if len(out) == 0 && e.DepartmentID != nil {
			team, err := r.Employees.ByDepartment(*e.DepartmentID, true)
			if err != nil {
				return nil, err
			}
			for _, p := range team {
				if p.ID != e.ID && hasRole(&p, model.RoleManager) {
					out = append(out, p)
				}
			}
		}
	}
	filtered := out[:0]
	for _, p := range out {
		if p.ID != e.ID {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func flowFor(r repository.Repos, e *model.Employee, half bool) ([]FlowStep, error) {
	var out []FlowStep
	for _, st := range stagesFor(e, half) {
		names, err := approverNames(r, e, st)
		if err != nil {
			return nil, err
		}
		out = append(out, FlowStep{Level: st.Level, Role: st.Kind.label(), Names: names})
	}
	return out, nil
}

func timelineFor(r repository.Repos, l *model.LeaveRequest) ([]TimelineStep, error) {
	e := l.Employee
	submitted := l.CreatedAt
	out := []TimelineStep{{Title: "Diajukan", Actor: e.Name, At: &submitted, State: "done"}}
	terminal := l.Status == model.LeaveRejected || l.Status == model.LeaveCancelled
	for _, st := range stagesFor(e, l.HalfDay) {
		var ap *model.LeaveApproval
		for i := range l.Approvals {
			if l.Approvals[i].Level == st.Level {
				ap = &l.Approvals[i]
			}
		}
		who := ""
		if ap != nil && ap.Approver != nil {
			who = ap.Approver.Name
		}
		switch {
		case ap != nil && ap.Action == model.ActionApproved:
			at := ap.ActedAt
			out = append(out, TimelineStep{Title: "Disetujui " + strings.ToLower(st.Kind.label()), Actor: who, Note: ap.Note, At: &at, State: "done"})
		case ap != nil && ap.Action == model.ActionRejected:
			at := ap.ActedAt
			out = append(out, TimelineStep{Title: "Ditolak " + strings.ToLower(st.Kind.label()), Actor: who, Note: ap.Note, At: &at, State: "rejected"})
		case l.Status == st.Status:
			names, err := approverNames(r, e, st)
			if err != nil {
				return nil, err
			}
			out = append(out, TimelineStep{Title: "Menunggu " + strings.ToLower(st.Kind.label()), Actor: strings.Join(names, ", "), State: "current"})
		case !terminal && l.Status != model.LeaveApproved:
			names, err := approverNames(r, e, st)
			if err != nil {
				return nil, err
			}
			out = append(out, TimelineStep{Title: "Persetujuan " + strings.ToLower(st.Kind.label()), Actor: strings.Join(names, ", "), State: "todo"})
		}
	}
	if l.Status == model.LeaveCancelled && l.CancelledAt != nil {
		out = append(out, TimelineStep{Title: "Dibatalkan", Actor: e.Name, At: l.CancelledAt, State: "cancelled"})
	}
	return out, nil
}

// detailView melengkapi baseView dengan linimasa, hari tak terhitung, saldo, dan bentrok tim.
func (s *LeaveService) detailView(r repository.Repos, a Actor, l *model.LeaveRequest) (*LeaveView, error) {
	v := s.baseView(a, l)
	tl, err := timelineFor(r, l)
	if err != nil {
		return nil, err
	}
	v.Timeline = tl
	if sch, err := scheduleFor(r, l.Employee); err == nil {
		var part *Partial
		if l.HalfDay {
			if pr, ok := partialOf(l, sch); ok {
				part = &pr
			}
		}
		if wd, err := s.wd.Count(r, sch, l.StartDate, l.EndDate, part); err == nil {
			v.Skipped = wd.Skipped
		}
	}
	if l.LeaveType != nil && trackedType(l.LeaveType) {
		year := DateOnly(l.StartDate).Year()
		bal, err := r.Balances.Ensure(l.EmployeeID, l.LeaveTypeID, year, *l.LeaveType.AnnualQuota)
		if err != nil {
			return nil, err
		}
		pending, err := r.Leaves.SumDays(l.EmployeeID, l.LeaveTypeID, year, pendingStatuses)
		if err != nil {
			return nil, err
		}
		if model.LeavePending(l.Status) {
			pending = pending.Sub(l.Days) // pengajuan ini dihitung terpisah
		}
		avail := bal.Quota.Sub(bal.Used).Sub(pending)
		after := avail.Sub(l.Days)
		if l.Status == model.LeaveApproved && l.Deducted {
			avail, after = bal.Quota.Sub(bal.Used).Add(l.Days).Sub(pending), bal.Quota.Sub(bal.Used).Sub(pending)
		}
		v.Balance = &BalanceInfo{Quota: bal.Quota, Used: bal.Used, Pending: pending, Available: avail, After: after}
	}
	if model.LeavePending(l.Status) && l.Employee.DepartmentID != nil && (v.CanAct || a.IsHR() || a.IsDirector()) {
		others, err := r.Leaves.Calendar(DateOnly(l.StartDate), DateOnly(l.EndDate), *l.Employee.DepartmentID,
			[]string{model.LeaveApproved, model.LeavePendingManager, model.LeavePendingHR})
		if err != nil {
			return nil, err
		}
		for _, o := range others {
			if o.ID == l.ID || o.EmployeeID == l.EmployeeID {
				continue
			}
			c := Conflict{Range: fmtRange(DateOnly(o.StartDate), DateOnly(o.EndDate)), Status: leaveStatusLabel[o.Status]}
			if o.Employee != nil {
				c.EmployeeName = o.Employee.Name
			}
			if o.LeaveType != nil {
				c.TypeName = o.LeaveType.Name
			}
			v.Conflicts = append(v.Conflicts, c)
		}
	}
	return &v, nil
}

// ---------- saldo ----------

type BalanceView struct {
	LeaveType model.LeaveType  `json:"leave_type"`
	Year      int              `json:"year"`
	Tracked   bool             `json:"tracked"`
	Quota     *decimal.Decimal `json:"quota"`
	Used      decimal.Decimal  `json:"used"`
	Pending   decimal.Decimal  `json:"pending"`
	Available *decimal.Decimal `json:"available"`
}

func (s *LeaveService) balances(r repository.Repos, e *model.Employee, year int) ([]BalanceView, error) {
	types, err := r.LeaveTypes.List(true)
	if err != nil {
		return nil, err
	}
	out := make([]BalanceView, 0, len(types))
	for _, t := range types {
		pending, err := r.Leaves.SumDays(e.ID, t.ID, year, pendingStatuses)
		if err != nil {
			return nil, err
		}
		bv := BalanceView{LeaveType: t, Year: year, Pending: pending}
		if trackedType(&t) {
			bal, err := r.Balances.Ensure(e.ID, t.ID, year, *t.AnnualQuota)
			if err != nil {
				return nil, err
			}
			avail := decimal.Max(bal.Quota.Sub(bal.Used).Sub(pending), decimal.Zero)
			q := bal.Quota
			bv.Tracked, bv.Quota, bv.Used, bv.Available = true, &q, bal.Used, &avail
		} else {
			used, err := r.Leaves.SumDays(e.ID, t.ID, year, approvedStatuses)
			if err != nil {
				return nil, err
			}
			bv.Used = used
		}
		out = append(out, bv)
	}
	return out, nil
}

// Balances: saldo per jenis untuk karyawan (default diri sendiri; manajer/HR/Direktur sesuai hak lihat).
func (s *LeaveService) Balances(a Actor, employeeID uint64, year int) ([]BalanceView, error) {
	if employeeID == 0 {
		employeeID = a.EmployeeID
	}
	r := s.store.R()
	e, err := loadEmployee(r, employeeID)
	if err != nil {
		return nil, err
	}
	if err := requireView(a, e); err != nil {
		return nil, err
	}
	if year == 0 {
		year = s.today().Year()
	}
	return s.balances(r, e, year)
}

// ---------- pratinjau & pengajuan ----------

type LeavePreview struct {
	WorkingDays *WorkingDays `json:"working_days"`
	Flow        []FlowStep   `json:"flow"`
	Balance     *BalanceInfo `json:"balance,omitempty"`
	Warnings    []string     `json:"warnings"`
}

// PartialSpec menyatakan izin sebagian hari: pilih sesi (pagi/siang) atau isi jam mulai–selesai ("HH:MM").
type PartialSpec struct {
	Half    bool
	Session string
	Start   string
	End     string
}

// resolvePartial menerjemahkan spec menjadi rentang jam pada hari date menurut jadwal sch.
// Mengembalikan (nil, "", nil) bila bukan izin sebagian hari; session terisi bila rentangnya tepat setengah hari pertama/kedua.
func resolvePartial(sch Schedule, date time.Time, sp PartialSpec) (*Partial, string, error) {
	if !sp.Half {
		return nil, "", nil
	}
	h, works := sch.Hours(date.Weekday())
	if !works {
		return nil, "", fieldErr("Izin sebagian hari hanya bisa pada hari kerja", "start_date")
	}
	me, as := sch.Halves(h)
	var p Partial
	if strings.TrimSpace(sp.Start) != "" || strings.TrimSpace(sp.End) != "" {
		a, e1 := parseHM(sp.Start)
		b, e2 := parseHM(sp.End)
		if e1 != nil {
			return nil, "", fieldErr("Jam mulai harus berformat HH:MM", "start_time")
		}
		if e2 != nil {
			return nil, "", fieldErr("Jam selesai harus berformat HH:MM", "end_time")
		}
		p = Partial{From: a, To: b}
	} else {
		switch sp.Session {
		case model.SessionAfternoon:
			p = Partial{From: as, To: h.End}
		case model.SessionMorning, "":
			p = Partial{From: h.Start, To: me}
		default:
			return nil, "", fieldErr("Sesi harus pagi atau siang", "half_session")
		}
	}
	switch {
	case p.To <= p.From:
		return nil, "", fieldErr("Jam selesai harus setelah jam mulai", "end_time")
	case p.From < h.Start || p.To > h.End:
		return nil, "", fieldErr(fmt.Sprintf("Jam di luar jam kerja hari itu (%s)", h), "start_time")
	case sch.Net(p.From, p.To) < 15:
		return nil, "", fieldErr("Izin sebagian hari minimal 15 menit di luar jam istirahat", "end_time")
	case p.From == h.Start && p.To == h.End:
		return nil, "", fieldErr("Seluruh jam kerja hari itu: ajukan sebagai satu hari penuh", "end_time")
	}
	session := ""
	switch {
	case p.From == h.Start && p.To == me:
		session = model.SessionMorning
	case p.From == as && p.To == h.End:
		session = model.SessionAfternoon
	}
	return &p, session, nil
}

// partialOf membaca rentang jam sebuah pengajuan sebagian hari (jam tersimpan, atau turunan dari sesi untuk data lama).
func partialOf(l *model.LeaveRequest, sch Schedule) (Partial, bool) {
	if !l.HalfDay {
		return Partial{}, false
	}
	if l.StartTime != nil && l.EndTime != nil {
		a, e1 := parseHM(*l.StartTime)
		b, e2 := parseHM(*l.EndTime)
		if e1 == nil && e2 == nil {
			return Partial{From: a, To: b}, true
		}
	}
	h, ok := sch.Hours(DateOnly(l.StartDate).Weekday())
	if !ok || l.HalfSession == nil {
		return Partial{}, false
	}
	me, as := sch.Halves(h)
	if *l.HalfSession == model.SessionAfternoon {
		return Partial{From: as, To: h.End}, true
	}
	return Partial{From: h.Start, To: me}, true
}

// Preview menghitung hari kerja secara live untuk form H-08 beserta alur persetujuan dan dampak ke saldo.
func (s *LeaveService) Preview(a Actor, typeID uint64, start, end time.Time, sp PartialSpec) (*LeavePreview, error) {
	r := s.store.R()
	emp, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	sch, err := scheduleFor(r, emp)
	if err != nil {
		return nil, err
	}
	part, _, err := resolvePartial(sch, DateOnly(start), sp)
	if err != nil {
		return nil, err
	}
	wd, err := s.wd.Count(r, sch, start, end, part)
	if err != nil {
		return nil, err
	}
	flow, err := flowFor(r, emp, sp.Half)
	if err != nil {
		return nil, err
	}
	p := &LeavePreview{WorkingDays: wd, Flow: flow, Warnings: []string{}}
	if overlap, err := s.overlap(r, emp.ID, sch, DateOnly(start), DateOnly(end), part, 0); err == nil && overlap != "" {
		p.Warnings = append(p.Warnings, overlap)
	}
	if typeID != 0 {
		t, err := r.LeaveTypes.ByID(typeID)
		if err == nil && trackedType(t) {
			year := DateOnly(start).Year()
			bal, err := r.Balances.Ensure(emp.ID, t.ID, year, *t.AnnualQuota)
			if err != nil {
				return nil, err
			}
			pending, err := r.Leaves.SumDays(emp.ID, t.ID, year, pendingStatuses)
			if err != nil {
				return nil, err
			}
			avail := bal.Quota.Sub(bal.Used).Sub(pending)
			p.Balance = &BalanceInfo{Quota: bal.Quota, Used: bal.Used, Pending: pending, Available: avail, After: avail.Sub(wd.Days)}
			if p.Balance.After.IsNegative() {
				p.Warnings = append(p.Warnings, fmt.Sprintf("Saldo %s tidak cukup: tersisa %s hari.", t.Name, fmtDays(avail)))
			}
		}
	}
	return p, nil
}

// overlap mengembalikan pesan bila tanggal bertabrakan dengan pengajuan lain yang masih berlaku.
// Dua izin sebagian hari pada tanggal yang sama boleh asal jamnya tidak beririsan.
func (s *LeaveService) overlap(r repository.Repos, empID uint64, sch Schedule, start, end time.Time, part *Partial, excludeID uint64) (string, error) {
	others, err := r.Leaves.Overlapping(empID, start, end, excludeID)
	if err != nil {
		return "", err
	}
	for _, o := range others {
		if part != nil && o.HalfDay && DateOnly(o.StartDate).Equal(start) {
			if or, ok := partialOf(&o, sch); ok && (part.To <= or.From || part.From >= or.To) {
				continue
			}
		}
		no := ""
		if o.Number != nil {
			no = *o.Number
		}
		return fmt.Sprintf("Tanggal tumpang tindih dengan pengajuan %s (%s)", no, fmtRange(DateOnly(o.StartDate), DateOnly(o.EndDate))), nil
	}
	return "", nil
}

type LeaveInput struct {
	LeaveTypeID  uint64
	StartDate    time.Time
	EndDate      time.Time
	HalfDay      bool
	HalfSession  string
	StartTime    string // izin sebagian hari: jam mulai "HH:MM" (opsional bila HalfSession dipakai)
	EndTime      string
	Reason       string
	SubstituteID *uint64
	Filename     string
	File         io.Reader // opsional
}

var leaveSniffExt = map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png"}

func readAttachment(name string, body io.Reader) (data []byte, ext string, err error) {
	data, err = io.ReadAll(io.LimitReader(body, MaxEvidenceBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", apperr.Validation("Berkas kosong", map[string]string{"attachment": "Berkas kosong"})
	}
	if len(data) > MaxEvidenceBytes {
		return nil, "", apperr.Validation("Berkas terlalu besar", map[string]string{"attachment": "Maksimal 5 MB"})
	}
	ext, ok := leaveSniffExt[http.DetectContentType(data)]
	if !ok {
		return nil, "", apperr.Validation("Jenis berkas tidak didukung", map[string]string{"attachment": "Hanya PDF, JPG, atau PNG"})
	}
	_ = name
	return data, ext, nil
}

func fieldErr(msg, field string) error { return apperr.Validation(msg, map[string]string{field: msg}) }

// Submit membuat pengajuan: memvalidasi jenis, tanggal, hari kerja, masa kerja, lampiran, saldo, dan tumpang tindih.
func (s *LeaveService) Submit(ctx context.Context, a Actor, in LeaveInput) (*LeaveView, error) {
	r := s.store.R()
	emp, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	t, err := r.LeaveTypes.ByID(in.LeaveTypeID)
	if err != nil || !t.IsActive {
		return nil, fieldErr("Pilih jenis izin/cuti yang tersedia", "leave_type_id")
	}
	reason := strings.TrimSpace(in.Reason)
	switch {
	case reason == "":
		return nil, fieldErr("Alasan wajib diisi", "reason")
	case len([]rune(reason)) > maxLeaveReasonRunes:
		return nil, fieldErr("Alasan maksimal 1000 karakter", "reason")
	}
	start, end := DateOnly(in.StartDate), DateOnly(in.EndDate)
	if start.IsZero() || end.IsZero() {
		return nil, fieldErr("Tanggal wajib diisi", "start_date")
	}
	half := in.HalfDay
	if half && !t.AllowHalfDay {
		return nil, fieldErr(t.Name+" tidak bisa diajukan sebagian hari", "half_day")
	}
	if start.Before(s.today().AddDate(0, 0, -maxBackdateDays)) {
		return nil, fieldErr(fmt.Sprintf("Tanggal mulai tidak boleh lebih dari %d hari ke belakang", maxBackdateDays), "start_date")
	}
	sch, err := scheduleFor(r, emp)
	if err != nil {
		return nil, err
	}
	part, session, err := resolvePartial(sch, start, PartialSpec{Half: half, Session: in.HalfSession, Start: in.StartTime, End: in.EndTime})
	if err != nil {
		return nil, err
	}
	wd, err := s.wd.Count(r, sch, start, end, part)
	if err != nil {
		return nil, err
	}
	days := wd.Days
	if !days.IsPositive() {
		return nil, fieldErr("Rentang tanggal tidak berisi hari kerja (akhir pekan/hari libur)", "start_date")
	}
	if t.MaxDaysPerRequest != nil && days.GreaterThan(*t.MaxDaysPerRequest) {
		return nil, fieldErr(fmt.Sprintf("%s maksimal %s hari kerja per pengajuan", t.Name, fmtDays(*t.MaxDaysPerRequest)), "end_date")
	}
	if t.MinTenureMonths > 0 && monthsBetween(DateOnly(emp.JoinDate), start) < t.MinTenureMonths {
		return nil, fieldErr(fmt.Sprintf("%s baru dapat diajukan setelah masa kerja %d bulan (masuk %s)",
			t.Name, t.MinTenureMonths, fmtDate(DateOnly(emp.JoinDate))), "leave_type_id")
	}
	if trackedType(t) && start.Year() != end.Year() {
		return nil, fieldErr("Pengajuan tidak boleh melintasi pergantian tahun; ajukan terpisah per tahun", "end_date")
	}

	needFile := t.RequiresAttach && days.GreaterThan(t.AttachMinDays)
	var data []byte
	var ext string
	if in.File != nil {
		if data, ext, err = readAttachment(in.Filename, in.File); err != nil {
			return nil, err
		}
	}
	if needFile && data == nil {
		msg := "Lampiran wajib diunggah untuk " + t.Name
		if t.AttachMinDays.IsPositive() {
			msg += " lebih dari " + fmtDays(t.AttachMinDays) + " hari"
		}
		return nil, fieldErr(msg, "attachment")
	}

	if in.SubstituteID != nil {
		sub, err := r.Employees.ByID(*in.SubstituteID)
		switch {
		case err != nil || sub.Status != model.StatusActive:
			return nil, fieldErr("Karyawan pengganti tidak ditemukan", "substitute_id")
		case sub.ID == emp.ID:
			return nil, fieldErr("Pengganti tidak boleh diri sendiri", "substitute_id")
		case emp.DepartmentID != nil && !sameDept(emp.DepartmentID, sub.DepartmentID):
			return nil, fieldErr("Pengganti harus dari departemen yang sama", "substitute_id")
		}
	}

	var savedPath string
	var out *model.LeaveRequest
	err = s.store.Tx(func(r repository.Repos) error {
		if msg, err := s.overlap(r, emp.ID, sch, start, end, part, 0); err != nil {
			return err
		} else if msg != "" {
			return fieldErr(msg, "start_date")
		}
		if trackedType(t) {
			year := start.Year()
			if _, err := r.Balances.Ensure(emp.ID, t.ID, year, *t.AnnualQuota); err != nil {
				return err
			}
			bal, err := r.Balances.Lock(emp.ID, t.ID, year)
			if err != nil {
				return err
			}
			pending, err := r.Leaves.SumDays(emp.ID, t.ID, year, pendingStatuses)
			if err != nil {
				return err
			}
			avail := bal.Quota.Sub(bal.Used).Sub(pending)
			if days.GreaterThan(avail) {
				return fieldErr(fmt.Sprintf("Saldo %s tidak cukup: tersisa %s hari, diminta %s hari",
					t.Name, fmtDays(decimal.Max(avail, decimal.Zero)), fmtDays(days)), "end_date")
			}
		}
		l := &model.LeaveRequest{
			EmployeeID: emp.ID, LeaveTypeID: t.ID, StartDate: start, EndDate: end, HalfDay: half,
			Days: days, Reason: reason, SubstituteID: in.SubstituteID, Status: firstStatus(emp, half),
		}
		if part != nil {
			st, et := fmtHM(part.From), fmtHM(part.To)
			l.StartTime, l.EndTime = &st, &et
			if session != "" {
				l.HalfSession = &session
			}
		}
		if data != nil {
			rel, err := s.files.Save(filepath.ToSlash(filepath.Join("leave", itoa64(emp.ID))), ext, bytes.NewReader(data))
			if err != nil {
				return err
			}
			savedPath = rel
			l.AttachmentPath, l.AttachmentName = rel, safeName(in.Filename, ext)
		}
		if err := r.Leaves.Create(l); err != nil {
			return err
		}
		no := fmt.Sprintf("%s-%d-%04d", t.Code, s.today().Year(), l.ID)
		l.Number = &no
		if err := r.Leaves.Save(l); err != nil {
			return err
		}
		out = l
		return logActivity(r, a, AuditLeaveSubmit, "leave_request", l.ID, nil,
			map[string]any{"number": no, "type": t.Name, "start": start.Format("2006-01-02"), "end": end.Format("2006-01-02"), "days": days.String()})
	})
	if err != nil {
		if savedPath != "" {
			_ = s.files.Delete(savedPath)
		}
		return nil, err
	}

	full, err := s.store.R().Leaves.ByID(out.ID)
	if err != nil {
		return nil, err
	}
	s.notifyStage(ctx, full, EvLeaveSubmitted, fmt.Sprintf("%s mengajukan %s %s (%s hari kerja). Menunggu persetujuan Anda.",
		emp.Name, t.Name, fmtRange(start, end), fmtDays(days)))
	v := s.baseView(a, full)
	return &v, nil
}

// notifyStage memberi tahu penyetuju tahap yang sedang berjalan.
func (s *LeaveService) notifyStage(ctx context.Context, l *model.LeaveRequest, event, msg string) {
	st, _, ok := currentStage(l)
	if !ok {
		return
	}
	to, err := approversOf(s.store.R(), l.Employee, st)
	if err != nil || len(to) == 0 {
		return
	}
	title := "Pengajuan " + l.LeaveType.Name + " — " + l.Employee.Name
	s.notifier.Notify(ctx, Notice{Type: event, Title: title, Message: msg, Link: "/persetujuan",
		Data: map[string]any{"leave_request_id": l.ID}}, to...)
}

func (s *LeaveService) notifyOwner(ctx context.Context, l *model.LeaveRequest, event, title, msg string) {
	s.notifier.Notify(ctx, Notice{Type: event, Title: title, Message: msg,
		Link: fmt.Sprintf("/izin-cuti/riwayat?id=%d", l.ID), Data: map[string]any{"leave_request_id": l.ID}}, *l.Employee)
}

func leaveNo(l *model.LeaveRequest) string {
	if l.Number != nil {
		return *l.Number
	}
	return fmt.Sprintf("#%d", l.ID)
}

// ---------- baca ----------

func (s *LeaveService) load(r repository.Repos, id uint64) (*model.LeaveRequest, error) {
	l, err := r.Leaves.ByID(id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, apperr.NotFound("Pengajuan tidak ditemukan")
		}
		return nil, err
	}
	return l, nil
}

// Get: detail pengajuan (pemilik, atasan, HR, Direktur).
func (s *LeaveService) Get(a Actor, id uint64) (*LeaveView, error) {
	r := s.store.R()
	l, err := s.load(r, id)
	if err != nil {
		return nil, err
	}
	if !canSeeLeave(a, l) {
		return nil, apperr.NotFound("Pengajuan tidak ditemukan") // jangan membocorkan keberadaan
	}
	return s.detailView(r, a, l)
}

type LeaveListFilter struct {
	EmployeeID uint64
	Status     string // "", "pending", "approved", "rejected", "cancelled"
	Year       int
	Page       repository.Page
}

func statusFilter(s string) []string {
	switch s {
	case "pending":
		return pendingStatuses
	case model.LeaveApproved, model.LeaveRejected, model.LeaveCancelled:
		return []string{s}
	case model.LeavePendingManager, model.LeavePendingHR:
		return []string{s}
	}
	return nil
}

// List: riwayat pengajuan satu karyawan (default diri sendiri).
func (s *LeaveService) List(a Actor, f LeaveListFilter) ([]LeaveView, int64, error) {
	r := s.store.R()
	if f.EmployeeID == 0 {
		f.EmployeeID = a.EmployeeID
	}
	e, err := loadEmployee(r, f.EmployeeID)
	if err != nil {
		return nil, 0, err
	}
	if err := requireView(a, e); err != nil {
		return nil, 0, err
	}
	rows, total, err := r.Leaves.List(repository.LeaveFilter{EmployeeID: e.ID, Statuses: statusFilter(f.Status), Year: f.Year, Page: f.Page})
	if err != nil {
		return nil, 0, err
	}
	out := make([]LeaveView, 0, len(rows))
	for i := range rows {
		out = append(out, s.baseView(a, &rows[i]))
	}
	return out, total, nil
}

// Queue: antrian persetujuan untuk a (tab "pending") atau riwayat keputusan a (tab "history").
func (s *LeaveService) Queue(a Actor, tab string) ([]LeaveView, error) {
	r := s.store.R()
	if !a.HasAny(model.RoleManager, model.RoleHR, model.RoleDirector) {
		return nil, apperr.Forbidden("Anda tidak memiliki akses persetujuan")
	}
	out := []LeaveView{}
	if tab == "history" {
		aps, err := r.Approvals.ByApprover(a.EmployeeID, 100)
		if err != nil {
			return nil, err
		}
		seen := map[uint64]bool{}
		for _, ap := range aps {
			if seen[ap.LeaveRequestID] {
				continue
			}
			seen[ap.LeaveRequestID] = true
			l, err := r.Leaves.ByID(ap.LeaveRequestID)
			if err != nil {
				continue
			}
			v := s.baseView(a, l)
			v.Timeline, _ = timelineFor(r, l)
			out = append(out, v)
		}
		return out, nil
	}
	rows, err := r.Leaves.ByStatuses(pendingStatuses, 300)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if canActOn(a, &rows[i]) {
			out = append(out, s.baseView(a, &rows[i]))
		}
	}
	return out, nil
}

// PendingCount: jumlah pengajuan yang menunggu keputusan a.
func (s *LeaveService) PendingCount(a Actor) (int, error) {
	if !a.HasAny(model.RoleManager, model.RoleHR, model.RoleDirector) {
		return 0, nil
	}
	rows, err := s.store.R().Leaves.ByStatuses(pendingStatuses, 300)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range rows {
		if canActOn(a, &rows[i]) {
			n++
		}
	}
	return n, nil
}

// Colleague adalah calon karyawan pengganti.
type Colleague struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position,omitempty"`
}

// Substitutes: rekan aktif satu departemen (kecuali diri sendiri) yang dapat dipilih sebagai pengganti.
func (s *LeaveService) Substitutes(a Actor) ([]Colleague, error) {
	r := s.store.R()
	e, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	out := []Colleague{}
	if e.DepartmentID == nil {
		return out, nil
	}
	team, err := r.Employees.ByDepartment(*e.DepartmentID, true)
	if err != nil {
		return nil, err
	}
	for _, p := range team {
		if p.ID == e.ID {
			continue
		}
		c := Colleague{ID: p.ID, Name: p.Name}
		if p.Position != nil {
			c.Position = p.Position.Name
		}
		out = append(out, c)
	}
	return out, nil
}

// FlowPreview: alur persetujuan untuk pengaju a (tampil di form).
func (s *LeaveService) FlowPreview(a Actor, half bool) ([]FlowStep, error) {
	r := s.store.R()
	e, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	return flowFor(r, e, half)
}

// ---------- ringkasan dasbor ----------

type LeaveSummary struct {
	Year         int           `json:"year"`
	Balances     []BalanceView `json:"balances"`
	Recent       []LeaveView   `json:"recent"`
	PendingCount int           `json:"pending_count"` // menunggu keputusan pengguna ini
	Queue        []LeaveView   `json:"queue"`         // beberapa teratas
}

func (s *LeaveService) Summary(a Actor) (*LeaveSummary, error) {
	r := s.store.R()
	e, err := loadEmployee(r, a.EmployeeID)
	if err != nil {
		return nil, err
	}
	year := s.today().Year()
	bals, err := s.balances(r, e, year)
	if err != nil {
		return nil, err
	}
	rows, _, err := r.Leaves.List(repository.LeaveFilter{EmployeeID: e.ID, Page: repository.Page{Page: 1, PerPage: 3}})
	if err != nil {
		return nil, err
	}
	sum := &LeaveSummary{Year: year, Balances: bals, Recent: []LeaveView{}, Queue: []LeaveView{}}
	for i := range rows {
		sum.Recent = append(sum.Recent, s.baseView(a, &rows[i]))
	}
	q, err := s.Queue(a, "pending")
	if err == nil {
		sum.PendingCount = len(q)
		if len(q) > 5 {
			q = q[:5]
		}
		sum.Queue = q
	}
	return sum, nil
}

// ---------- persetujuan ----------

func (s *LeaveService) decide(ctx context.Context, a Actor, id uint64, approve bool, note string) (*LeaveView, error) {
	note = strings.TrimSpace(note)
	if !approve && note == "" {
		return nil, fieldErr("Alasan penolakan wajib diisi", "note")
	}
	if len([]rune(note)) > 500 {
		return nil, fieldErr("Catatan maksimal 500 karakter", "note")
	}
	var after *model.LeaveRequest
	var prevStatus string
	var movedOn bool
	err := s.store.Tx(func(r repository.Repos) error {
		l, err := r.Leaves.Lock(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Pengajuan tidak ditemukan")
			}
			return err
		}
		if !canSeeLeave(a, l) {
			return apperr.NotFound("Pengajuan tidak ditemukan")
		}
		if !model.LeavePending(l.Status) {
			return apperr.Conflict("Pengajuan sudah diproses (" + leaveStatusLabel[l.Status] + ")")
		}
		st, idx, _ := currentStage(l)
		if !canActOn(a, l) {
			return apperr.Forbidden("Anda bukan penyetuju pada tahap ini")
		}
		prevStatus = l.Status
		action := model.ActionRejected
		if approve {
			action = model.ActionApproved
		}
		if err := r.Approvals.Create(&model.LeaveApproval{LeaveRequestID: l.ID, ApproverID: a.EmployeeID,
			Level: st.Level, Action: action, Note: note, ActedAt: s.now()}); err != nil {
			return err
		}
		stages := stagesFor(l.Employee, l.HalfDay)
		switch {
		case !approve:
			l.Status = model.LeaveRejected
		case idx+1 < len(stages):
			l.Status, movedOn = stages[idx+1].Status, true
		default:
			if err := s.deduct(r, l); err != nil {
				return err
			}
			l.Status = model.LeaveApproved
		}
		if err := r.Leaves.Save(l); err != nil {
			return err
		}
		audit := AuditLeaveApprove
		if !approve {
			audit = AuditLeaveReject
		}
		after = l
		return logActivity(r, a, audit, "leave_request", l.ID,
			map[string]any{"status": prevStatus}, map[string]any{"status": l.Status, "level": st.Level, "note": note})
	})
	if err != nil {
		return nil, err
	}

	full, err := s.store.R().Leaves.ByID(after.ID)
	if err != nil {
		return nil, err
	}
	s.notifyDecision(ctx, a, full, approve, movedOn, note)
	return s.detailView(s.store.R(), a, full)
}

// deduct memotong saldo (terkunci) saat persetujuan terakhir; menolak bila saldo tak lagi cukup.
func (s *LeaveService) deduct(r repository.Repos, l *model.LeaveRequest) error {
	t := l.LeaveType
	if t == nil || !trackedType(t) || l.Deducted {
		return nil
	}
	year := DateOnly(l.StartDate).Year()
	if _, err := r.Balances.Ensure(l.EmployeeID, t.ID, year, *t.AnnualQuota); err != nil {
		return err
	}
	bal, err := r.Balances.Lock(l.EmployeeID, t.ID, year)
	if err != nil {
		return err
	}
	if bal.Used.Add(l.Days).GreaterThan(bal.Quota) {
		return apperr.Conflict(fmt.Sprintf("Saldo %s tidak cukup saat persetujuan: sisa %s hari, diminta %s hari. Tolak pengajuan atau sesuaikan kuota.",
			t.Name, fmtDays(bal.Quota.Sub(bal.Used)), fmtDays(l.Days)))
	}
	bal.Used = bal.Used.Add(l.Days)
	if err := r.Balances.Save(bal); err != nil {
		return err
	}
	l.Deducted = true
	return nil
}

func (s *LeaveService) refund(r repository.Repos, l *model.LeaveRequest) error {
	if !l.Deducted || l.LeaveType == nil {
		return nil
	}
	bal, err := r.Balances.Lock(l.EmployeeID, l.LeaveTypeID, DateOnly(l.StartDate).Year())
	if err != nil {
		return err
	}
	bal.Used = decimal.Max(bal.Used.Sub(l.Days), decimal.Zero)
	if err := r.Balances.Save(bal); err != nil {
		return err
	}
	l.Deducted = false
	return nil
}

func (s *LeaveService) notifyDecision(ctx context.Context, a Actor, l *model.LeaveRequest, approve, movedOn bool, note string) {
	desc := fmt.Sprintf("%s %s (%s hari kerja)", l.LeaveType.Name, fmtRange(DateOnly(l.StartDate), DateOnly(l.EndDate)), fmtDays(l.Days))
	switch {
	case !approve:
		msg := fmt.Sprintf("Pengajuan %s — %s — ditolak oleh %s.", leaveNo(l), desc, a.Name)
		if note != "" {
			msg += " Alasan: " + note
		}
		s.notifyOwner(ctx, l, EvLeaveRejected, "Pengajuan ditolak", msg)
	case movedOn:
		s.notifyOwner(ctx, l, EvLeaveProgress, "Pengajuan diteruskan ke tahap berikutnya",
			fmt.Sprintf("Pengajuan %s — %s — disetujui oleh %s dan diteruskan ke tahap berikutnya.", leaveNo(l), desc, a.Name))
		s.notifyStage(ctx, l, EvLeaveSubmitted, fmt.Sprintf("Pengajuan %s dari %s (%s) sudah disetujui %s dan menunggu persetujuan Anda.",
			leaveNo(l), l.Employee.Name, desc, a.Name))
	default:
		s.notifyOwner(ctx, l, EvLeaveApproved, "Pengajuan disetujui",
			fmt.Sprintf("Pengajuan %s — %s — disetujui oleh %s.", leaveNo(l), desc, a.Name))
	}
}

func (s *LeaveService) Approve(ctx context.Context, a Actor, id uint64, note string) (*LeaveView, error) {
	return s.decide(ctx, a, id, true, note)
}

func (s *LeaveService) Reject(ctx context.Context, a Actor, id uint64, note string) (*LeaveView, error) {
	return s.decide(ctx, a, id, false, note)
}

// ---------- pembatalan ----------

// Cancel: pemilik membatalkan selama tanggal mulai belum tiba; saldo yang sudah dipotong dikembalikan.
func (s *LeaveService) Cancel(ctx context.Context, a Actor, id uint64) (*LeaveView, error) {
	var l *model.LeaveRequest
	var prev string
	var wasPending bool
	err := s.store.Tx(func(r repository.Repos) error {
		cur, err := r.Leaves.Lock(id)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Pengajuan tidak ditemukan")
			}
			return err
		}
		if cur.EmployeeID != a.EmployeeID {
			if !canSeeLeave(a, cur) {
				return apperr.NotFound("Pengajuan tidak ditemukan")
			}
			return apperr.Forbidden("Hanya pengaju yang dapat membatalkan pengajuan")
		}
		if !model.LeavePending(cur.Status) && cur.Status != model.LeaveApproved {
			return apperr.Conflict("Pengajuan berstatus " + leaveStatusLabel[cur.Status] + " tidak dapat dibatalkan")
		}
		if !s.today().Before(DateOnly(cur.StartDate)) {
			return apperr.Conflict("Pengajuan hanya dapat dibatalkan sebelum tanggal mulai (" + fmtDate(DateOnly(cur.StartDate)) + ")")
		}
		prev, wasPending = cur.Status, model.LeavePending(cur.Status)
		if err := s.refund(r, cur); err != nil {
			return err
		}
		now := s.now()
		cur.Status, cur.CancelledAt = model.LeaveCancelled, &now
		if err := r.Leaves.Save(cur); err != nil {
			return err
		}
		l = cur
		return logActivity(r, a, AuditLeaveCancel, "leave_request", cur.ID,
			map[string]any{"status": prev}, map[string]any{"status": model.LeaveCancelled})
	})
	if err != nil {
		return nil, err
	}
	full, err := s.store.R().Leaves.ByID(l.ID)
	if err != nil {
		return nil, err
	}
	// beri tahu yang sedang/ sudah menyetujui
	rr := s.store.R()
	var to []model.Employee
	if wasPending {
		l2 := *full
		l2.Status = prev
		if st, _, ok := currentStage(&l2); ok {
			to, _ = approversOf(rr, full.Employee, st)
		}
	} else {
		for _, ap := range full.Approvals {
			if ap.Approver != nil {
				to = append(to, *ap.Approver)
			}
		}
	}
	s.notifier.Notify(ctx, Notice{Type: EvLeaveCancelled, Title: "Pengajuan dibatalkan",
		Message: fmt.Sprintf("%s membatalkan pengajuan %s (%s %s).", full.Employee.Name, leaveNo(full), full.LeaveType.Name,
			fmtRange(DateOnly(full.StartDate), DateOnly(full.EndDate))),
		Link: "/persetujuan", Data: map[string]any{"leave_request_id": full.ID}}, to...)
	return s.detailView(rr, a, full)
}

// ---------- lampiran ----------

// OpenAttachment membuka lampiran bila a berhak melihat pengajuan.
func (s *LeaveService) OpenAttachment(a Actor, id uint64) (*os.File, string, error) {
	l, err := s.load(s.store.R(), id)
	if err != nil {
		return nil, "", err
	}
	if !canSeeLeave(a, l) {
		return nil, "", apperr.NotFound("Pengajuan tidak ditemukan")
	}
	if l.AttachmentPath == "" {
		return nil, "", apperr.NotFound("Pengajuan ini tidak memiliki lampiran")
	}
	f, err := s.files.Open(l.AttachmentPath)
	if err != nil {
		return nil, "", apperr.NotFound("Berkas lampiran tidak ditemukan")
	}
	return f, l.AttachmentName, nil
}

// ---------- kalender tim ----------

type CalendarItem struct {
	ID             uint64 `json:"id"`
	EmployeeID     uint64 `json:"employee_id"`
	EmployeeName   string `json:"employee_name"`
	DepartmentID   uint64 `json:"department_id"`
	DepartmentName string `json:"department_name"`
	TypeCode       string `json:"type_code"`
	TypeName       string `json:"type_name"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	HalfDay        bool   `json:"half_day"`
	HalfSession    string `json:"half_session,omitempty"`
	StartTime      string `json:"start_time,omitempty"`
	EndTime        string `json:"end_time,omitempty"`
	Status         string `json:"status"`
	Days           string `json:"days"`
}

type CalendarView struct {
	Month    string          `json:"month"` // YYYY-MM
	From     string          `json:"from"`
	To       string          `json:"to"`
	Items    []CalendarItem  `json:"items"`
	Holidays []model.Holiday `json:"holidays"`
}

// Calendar: izin/cuti yang disetujui (dan opsional yang masih menunggu) pada satu bulan; semua role boleh melihat.
// Alasan dan lampiran tidak pernah ikut tampil.
func (s *LeaveService) Calendar(a Actor, month time.Time, deptID uint64, includePending bool) (*CalendarView, error) {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	statuses := []string{model.LeaveApproved}
	if includePending {
		statuses = append(statuses, pendingStatuses...)
	}
	r := s.store.R()
	rows, err := r.Leaves.Calendar(first, last, deptID, statuses)
	if err != nil {
		return nil, err
	}
	hs, err := r.Holidays.InRange(first, last)
	if err != nil {
		return nil, err
	}
	v := &CalendarView{Month: first.Format("2006-01"), From: first.Format("2006-01-02"), To: last.Format("2006-01-02"),
		Items: make([]CalendarItem, 0, len(rows)), Holidays: hs}
	for _, l := range rows {
		it := CalendarItem{ID: l.ID, EmployeeID: l.EmployeeID, StartDate: DateOnly(l.StartDate).Format("2006-01-02"),
			EndDate: DateOnly(l.EndDate).Format("2006-01-02"), HalfDay: l.HalfDay, Status: l.Status, Days: fmtDays(l.Days)}
		if l.HalfSession != nil {
			it.HalfSession = *l.HalfSession
		}
		if l.StartTime != nil && l.EndTime != nil {
			it.StartTime, it.EndTime = *l.StartTime, *l.EndTime
		}
		if l.Employee != nil {
			it.EmployeeName = l.Employee.Name
			if l.Employee.Department != nil {
				it.DepartmentID, it.DepartmentName = l.Employee.Department.ID, l.Employee.Department.Name
			}
		}
		if l.LeaveType != nil {
			it.TypeCode, it.TypeName = l.LeaveType.Code, l.LeaveType.Name
		}
		v.Items = append(v.Items, it)
	}
	return v, nil
}
