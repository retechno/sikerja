package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/model"
)

// ---------- alur persetujuan (murni, tanpa DB) ----------

// approverKind: siapa yang menyetujui sebuah tahap.
type approverKind int

const (
	kindManager  approverKind = iota // atasan langsung (atau manajer departemen)
	kindDirector                     // Direktur
	kindHR                           // HR
)

func (k approverKind) label() string {
	switch k {
	case kindDirector:
		return "Direktur"
	case kindHR:
		return "HR"
	}
	return "Atasan langsung"
}

type stage struct {
	Level  int
	Status string // status pengajuan selama tahap ini berjalan
	Kind   approverKind
}

func hasRole(e *model.Employee, role string) bool {
	if e == nil || e.User == nil {
		return false
	}
	for _, r := range e.User.Roles {
		if r.Name == role {
			return true
		}
	}
	return false
}

// stagesFor menyusun tahap persetujuan untuk pengaju e:
//   - karyawan: atasan → HR; manajer: Direktur → HR;
//   - Direktur: langsung HR; HR: langsung Direktur (HR tidak menyetujui pengajuannya sendiri);
//   - izin setengah hari: cukup tahap pertama.
func stagesFor(e *model.Employee, half bool) []stage {
	var st []stage
	switch {
	case hasRole(e, model.RoleDirector):
		st = []stage{{2, model.LeavePendingHR, kindHR}}
	case hasRole(e, model.RoleHR):
		st = []stage{{1, model.LeavePendingManager, kindDirector}}
	case hasRole(e, model.RoleManager):
		st = []stage{{1, model.LeavePendingManager, kindDirector}, {2, model.LeavePendingHR, kindHR}}
	default:
		st = []stage{{1, model.LeavePendingManager, kindManager}, {2, model.LeavePendingHR, kindHR}}
	}
	if half && len(st) > 1 {
		st = st[:1]
	}
	return st
}

func firstStatus(e *model.Employee, half bool) string { return stagesFor(e, half)[0].Status }

// currentStage mengembalikan tahap yang sedang berjalan untuk status l, dan indeksnya.
func currentStage(l *model.LeaveRequest) (stage, int, bool) {
	for i, s := range stagesFor(l.Employee, l.HalfDay) {
		if s.Status == l.Status {
			return s, i, true
		}
	}
	return stage{}, 0, false
}

// canActOn: apakah a berhak menyetujui/menolak l pada tahap yang sedang berjalan.
func canActOn(a Actor, l *model.LeaveRequest) bool {
	e := l.Employee
	if e == nil || a.EmployeeID == e.ID || !model.LeavePending(l.Status) {
		return false
	}
	st, _, ok := currentStage(l)
	if !ok {
		return false
	}
	switch st.Kind {
	case kindDirector:
		return a.IsDirector()
	case kindHR:
		return a.IsHR()
	}
	if e.ManagerID != nil && *e.ManagerID == a.EmployeeID && a.HasAny(model.RoleManager, model.RoleDirector) {
		return true
	}
	return a.IsManager() && sameDept(a.DepartmentID, e.DepartmentID)
}

// canSeeLeave: pemilik, HR, Direktur, manajer departemen, atau atasan langsung.
func canSeeLeave(a Actor, l *model.LeaveRequest) bool {
	e := l.Employee
	if e == nil {
		return false
	}
	return a.CanView(e) || (e.ManagerID != nil && *e.ManagerID == a.EmployeeID) || canActOn(a, l)
}

// ---------- hitung bantu ----------

// monthsBetween: selisih bulan penuh dari from sampai to.
func monthsBetween(from, to time.Time) int {
	m := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
	if to.Day() < from.Day() {
		m--
	}
	return m
}

// trackedType: jenis yang saldo tahunannya dikelola (dipotong saat disetujui).
func trackedType(t *model.LeaveType) bool { return t.DeductsBalance && t.AnnualQuota != nil }

var leaveStatusLabel = map[string]string{
	model.LeavePendingManager: "Menunggu Atasan",
	model.LeavePendingHR:      "Menunggu HR",
	model.LeaveApproved:       "Disetujui",
	model.LeaveRejected:       "Ditolak",
	model.LeaveCancelled:      "Dibatalkan",
}

// ---------- format teks Indonesia ----------

var monthShort = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func fmtDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), monthShort[t.Month()-1], t.Year())
}

// fmtRange: "30 Okt – 2 Nov 2026", "4 – 6 Nov 2026", atau "9 Okt 2026" bila satu hari.
func fmtRange(a, b time.Time) string {
	switch {
	case a.Equal(b):
		return fmtDate(a)
	case a.Year() == b.Year() && a.Month() == b.Month():
		return fmt.Sprintf("%d – %s", a.Day(), fmtDate(b))
	case a.Year() == b.Year():
		return fmt.Sprintf("%d %s – %s", a.Day(), monthShort[a.Month()-1], fmtDate(b))
	}
	return fmtDate(a) + " – " + fmtDate(b)
}

// fmtDays: desimal pakai koma ("0,5", "2").
func fmtDays(d decimal.Decimal) string {
	t := d.StringFixed(2)
	if strings.Contains(t, ".") {
		t = strings.TrimRight(strings.TrimRight(t, "0"), ".")
	}
	return strings.Replace(t, ".", ",", 1)
}
