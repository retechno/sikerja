package service

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
	"github.com/retechno/sikerja/backend/internal/storage"
)

const MaxEvidenceBytes = 5 << 20 // 5 MB

type KpiService struct {
	store repository.Store
	files storage.Store
	cache *Cache
	calc  KpiScoreService
}

func NewKpiService(store repository.Store, files storage.Store, cache *Cache) *KpiService {
	return &KpiService{store: store, files: files, cache: cache}
}

// ---------- lembar KPI (satu karyawan, satu periode) ----------

type KpiRow struct {
	Indicator   model.KpiIndicator `json:"indicator"`
	Result      *model.KpiResult   `json:"result"`
	Achievement decimal.Decimal    `json:"achievement"` // 0–120
	Points      decimal.Decimal    `json:"points"`      // capaian × bobot
	Status      string             `json:"status"`      // missing | submitted | validated | rejected
	HasEvidence bool               `json:"has_evidence"`
}

type KpiSheet struct {
	Employee     *model.Employee        `json:"employee"`
	Period       *model.AppraisalPeriod `json:"period"`
	TemplateID   uint64                 `json:"template_id"`
	Rows         []KpiRow               `json:"rows"`
	Score        decimal.Decimal        `json:"score"` // skor KPI sementara
	Total        int                    `json:"total"`
	Validated    int                    `json:"validated"`
	Pending      int                    `json:"pending"` // menunggu validasi
	Rejected     int                    `json:"rejected"`
	Missing      int                    `json:"missing"`
	AllValidated bool                   `json:"all_validated"`
	CanInput     bool                   `json:"can_input"`
	CanValidate  bool                   `json:"can_validate"`
}

// assembleSheet menyusun lembar KPI dari template + hasil (fungsi murni; dipakai juga oleh dasbor massal).
// Skor sementara menghitung semua realisasi yang tidak ditolak; finalisasi mewajibkan semuanya tervalidasi.
func assembleSheet(calc KpiScoreService, emp *model.Employee, p *model.AppraisalPeriod, tpl *model.KpiTemplate, results []model.KpiResult) (*KpiSheet, error) {
	sh := &KpiSheet{Employee: emp, Period: p, Score: decimal.Zero}
	if tpl == nil || len(tpl.Indicators) == 0 {
		return sh, nil
	}
	sh.TemplateID = tpl.ID
	byInd := map[uint64]*model.KpiResult{}
	for i := range results {
		byInd[results[i].IndicatorID] = &results[i]
	}
	inputs := make([]IndicatorInput, 0, len(tpl.Indicators))
	for _, ind := range tpl.Indicators {
		in := IndicatorInput{IndicatorID: ind.ID, Target: ind.Target, Weight: ind.Weight, Direction: ind.Direction}
		if res := byInd[ind.ID]; res != nil && res.Status != model.ResultRejected {
			a := res.Actual
			in.Actual = &a
		}
		inputs = append(inputs, in)
	}
	scored, err := calc.Score(inputs)
	if err != nil {
		return nil, err
	}
	sh.Score = scored.Score
	for i, ind := range tpl.Indicators {
		res := byInd[ind.ID]
		row := KpiRow{Indicator: ind, Result: res, Achievement: scored.Lines[i].Achievement, Points: scored.Lines[i].Weighted, Status: "missing"}
		if res != nil {
			row.Status = res.Status
			row.HasEvidence = res.EvidencePath != ""
		}
		switch row.Status {
		case model.ResultValidated:
			sh.Validated++
		case model.ResultSubmitted:
			sh.Pending++
		case model.ResultRejected:
			sh.Rejected++
		default:
			sh.Missing++
		}
		sh.Rows = append(sh.Rows, row)
	}
	sh.Total = len(sh.Rows)
	sh.AllValidated = sh.Total > 0 && sh.Validated == sh.Total
	return sh, nil
}

func loadSheet(r repository.Repos, calc KpiScoreService, emp *model.Employee, p *model.AppraisalPeriod) (*KpiSheet, error) {
	if emp.DepartmentID == nil {
		return &KpiSheet{Employee: emp, Period: p, Score: decimal.Zero}, nil
	}
	tpl, err := r.Templates.ActiveByDepartment(*emp.DepartmentID)
	if err != nil && err != repository.ErrNotFound {
		return nil, err
	}
	res, err := r.Results.ForEmployee(p.ID, emp.ID)
	if err != nil {
		return nil, err
	}
	return assembleSheet(calc, emp, p, tpl, res)
}

// appraisalLocked true bila penilaian karyawan pada periode itu sudah final.
func appraisalLocked(r repository.Repos, periodID, empID uint64) (bool, error) {
	a, err := r.Appraisals.Find(periodID, empID)
	if err == repository.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return a.Status == model.AppraisalFinal, nil
}

// Sheet mengembalikan lembar KPI; hanya diri sendiri / manajer timnya / HR / Direktur.
func (s *KpiService) Sheet(a Actor, employeeID, periodID uint64) (*KpiSheet, error) {
	r := s.store.R()
	emp, err := loadEmployee(r, employeeID)
	if err != nil {
		return nil, err
	}
	if err := requireView(a, emp); err != nil {
		return nil, err
	}
	p, err := loadPeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PeriodDraft && !a.IsHR() {
		return nil, apperr.NotFound("Periode tidak ditemukan")
	}
	sh, err := loadSheet(r, s.calc, emp, p)
	if err != nil {
		return nil, err
	}
	locked, err := appraisalLocked(r, p.ID, emp.ID)
	if err != nil {
		return nil, err
	}
	open := p.Status == model.PeriodActive && !locked
	sh.CanInput = open && (emp.ID == a.EmployeeID || a.CanAppraise(emp))
	sh.CanValidate = open && a.CanAppraise(emp)
	return sh, nil
}

// ---------- input realisasi ----------

type SaveResultInput struct {
	PeriodID    uint64          `json:"period_id"`
	EmployeeID  uint64          `json:"employee_id"`
	IndicatorID uint64          `json:"indicator_id"`
	Actual      decimal.Decimal `json:"actual"`
	Note        string          `json:"note"`
}

// SaveResult: karyawan menginput realisasinya sendiri (status submitted, menunggu validasi);
// atasan yang menginput untuk anggota timnya langsung berstatus validated.
func (s *KpiService) SaveResult(a Actor, in SaveResultInput) (*model.KpiResult, error) {
	if in.Actual.IsNegative() {
		return nil, apperr.Validation("Realisasi tidak valid", map[string]string{"actual": "Tidak boleh negatif"})
	}
	if utf8.RuneCountInString(in.Note) > 2000 {
		return nil, apperr.Validation("Keterangan terlalu panjang", map[string]string{"note": "Maksimal 2000 karakter"})
	}
	var out *model.KpiResult
	err := s.store.Tx(func(r repository.Repos) error {
		emp, err := loadEmployee(r, in.EmployeeID)
		if err != nil {
			return err
		}
		self := emp.ID == a.EmployeeID
		if !self {
			if err := requireAppraise(a, emp); err != nil {
				return err
			}
		}
		p, err := loadPeriod(r, in.PeriodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		if locked, err := appraisalLocked(r, p.ID, emp.ID); err != nil {
			return err
		} else if locked {
			return apperr.Locked("Penilaian karyawan ini sudah final")
		}
		ind, err := s.indicatorForEmployee(r, emp, in.IndicatorID)
		if err != nil {
			return err
		}
		ach, err := s.calc.Achievement(in.Actual, ind.Target, ind.Direction)
		if err != nil {
			return apperr.Validation("Realisasi tidak valid", map[string]string{"actual": err.Error()})
		}

		old, err := r.Results.Find(p.ID, emp.ID, ind.ID)
		if err != nil && err != repository.ErrNotFound {
			return err
		}
		now := time.Now()
		uid := a.EmployeeID
		row := &model.KpiResult{PeriodID: p.ID, EmployeeID: emp.ID, IndicatorID: ind.ID, Actual: in.Actual,
			Achievement: ach, Note: strings.TrimSpace(in.Note), Status: model.ResultSubmitted, SubmittedBy: &uid}
		if !self { // diinput atasan → langsung tervalidasi
			row.Status, row.ValidatedBy, row.ValidatedAt = model.ResultValidated, &uid, &now
		}
		if err := r.Results.Upsert(row); err != nil {
			return err
		}
		out, err = r.Results.Find(p.ID, emp.ID, ind.ID)
		if err != nil {
			return err
		}
		if old != nil {
			return logActivity(r, a, AuditKpiChange, "kpi_result", out.ID,
				map[string]any{"actual": old.Actual, "status": old.Status},
				map[string]any{"actual": out.Actual, "status": out.Status})
		}
		return nil
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return out, err
}

func (s *KpiService) indicatorForEmployee(r repository.Repos, emp *model.Employee, indicatorID uint64) (*model.KpiIndicator, error) {
	if emp.DepartmentID == nil {
		return nil, apperr.BadRequest("Karyawan ini tidak memiliki KPI")
	}
	tpl, err := r.Templates.ActiveByDepartment(*emp.DepartmentID)
	if err != nil {
		return nil, apperr.BadRequest("Departemen belum memiliki template KPI aktif")
	}
	for i := range tpl.Indicators {
		if tpl.Indicators[i].ID == indicatorID {
			ind := tpl.Indicators[i]
			if ind.Source == model.SourceIntegration {
				return nil, apperr.BadRequest("Indikator ini diisi otomatis dari integrasi")
			}
			return &ind, nil
		}
	}
	return nil, apperr.NotFound("Indikator tidak ditemukan pada template karyawan ini")
}

// ---------- validasi ----------

const (
	ActionValidate = "validate"
	ActionReject   = "reject"
)

// Validate: atasan menyetujui/menolak realisasi anggota timnya (tercatat di audit log).
func (s *KpiService) Validate(a Actor, resultID uint64, action, note string) (*model.KpiResult, error) {
	if action != ActionValidate && action != ActionReject {
		return nil, apperr.Validation("Aksi tidak dikenal", map[string]string{"action": "Gunakan validate atau reject"})
	}
	note = strings.TrimSpace(note)
	if action == ActionReject && note == "" {
		return nil, apperr.Validation("Alasan penolakan wajib diisi", map[string]string{"note": "Wajib diisi"})
	}
	var out *model.KpiResult
	err := s.store.Tx(func(r repository.Repos) error {
		res, err := r.Results.ByID(resultID)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Realisasi tidak ditemukan")
			}
			return err
		}
		emp, err := loadEmployee(r, res.EmployeeID)
		if err != nil {
			return err
		}
		if err := requireAppraise(a, emp); err != nil {
			return err
		}
		p, err := loadPeriod(r, res.PeriodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		if locked, err := appraisalLocked(r, p.ID, emp.ID); err != nil {
			return err
		} else if locked {
			return apperr.Locked("Penilaian karyawan ini sudah final")
		}
		if res.Status == model.ResultValidated && action == ActionValidate {
			return apperr.Conflict("Realisasi sudah divalidasi")
		}
		old := map[string]any{"status": res.Status, "actual": res.Actual}
		now := time.Now()
		uid := a.EmployeeID
		res.ValidatedBy, res.ValidatedAt, res.ValidationNote = &uid, &now, note
		act := AuditKpiValidate
		if action == ActionValidate {
			res.Status = model.ResultValidated
		} else {
			res.Status = model.ResultRejected
			act = AuditKpiReject
		}
		if err := r.Results.Save(res); err != nil {
			return err
		}
		out = res
		return logActivity(r, a, act, "kpi_result", res.ID, old, map[string]any{"status": res.Status, "actual": res.Actual, "note": note})
	})
	if err == nil {
		s.cache.Bump(bgCtx())
	}
	return out, err
}

// ---------- bukti ----------

var sniffExt = map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png"}

// SaveEvidence menyimpan satu berkas bukti per realisasi (mengganti yang lama).
// Jenis ditentukan dari isi berkas, bukan dari ekstensi yang dikirim.
func (s *KpiService) SaveEvidence(a Actor, resultID uint64, filename string, body io.Reader) (*model.KpiResult, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxEvidenceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, apperr.Validation("Berkas kosong", map[string]string{"file": "Berkas kosong"})
	}
	if len(data) > MaxEvidenceBytes {
		return nil, apperr.Validation("Berkas terlalu besar", map[string]string{"file": "Maksimal 5 MB"})
	}
	ext, ok := sniffExt[http.DetectContentType(data)]
	if !ok {
		return nil, apperr.Validation("Jenis berkas tidak didukung", map[string]string{"file": "Hanya PDF, JPG, atau PNG"})
	}

	var out *model.KpiResult
	var oldPath string
	err = s.store.Tx(func(r repository.Repos) error {
		res, err := r.Results.ByID(resultID)
		if err != nil {
			if err == repository.ErrNotFound {
				return apperr.NotFound("Realisasi tidak ditemukan")
			}
			return err
		}
		emp, err := loadEmployee(r, res.EmployeeID)
		if err != nil {
			return err
		}
		self := emp.ID == a.EmployeeID
		if !self {
			if err := requireAppraise(a, emp); err != nil {
				return err
			}
		}
		p, err := loadPeriod(r, res.PeriodID)
		if err != nil {
			return err
		}
		if err := requireOpen(p); err != nil {
			return err
		}
		if locked, err := appraisalLocked(r, p.ID, emp.ID); err != nil {
			return err
		} else if locked {
			return apperr.Locked("Penilaian karyawan ini sudah final")
		}
		rel, err := s.files.Save(filepath.ToSlash(filepath.Join("evidence", itoa64(p.ID), itoa64(emp.ID))), ext, bytes.NewReader(data))
		if err != nil {
			return err
		}
		oldPath = res.EvidencePath
		res.EvidencePath, res.EvidenceName = rel, safeName(filename, ext)
		if self && res.Status == model.ResultValidated { // bukti diganti → perlu divalidasi ulang
			res.Status, res.ValidatedBy, res.ValidatedAt = model.ResultSubmitted, nil, nil
		}
		if err := r.Results.Save(res); err != nil {
			_ = s.files.Delete(rel)
			return err
		}
		out = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	if oldPath != "" {
		_ = s.files.Delete(oldPath)
	}
	s.cache.Bump(bgCtx())
	return out, nil
}

func itoa64(v uint64) string { return decimal.NewFromInt(int64(v)).String() }

func safeName(name, ext string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "" || name == "/" {
		name = "bukti" + ext
	}
	if utf8.RuneCountInString(name) > 150 {
		name = string([]rune(name)[:150])
	}
	return name
}

// OpenEvidence membuka berkas bukti bila actor berhak melihat data karyawan pemiliknya.
func (s *KpiService) OpenEvidence(a Actor, resultID uint64) (*os.File, string, error) {
	r := s.store.R()
	res, err := r.Results.ByID(resultID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, "", apperr.NotFound("Realisasi tidak ditemukan")
		}
		return nil, "", err
	}
	emp, err := loadEmployee(r, res.EmployeeID)
	if err != nil {
		return nil, "", err
	}
	if err := requireView(a, emp); err != nil {
		return nil, "", err
	}
	if res.EvidencePath == "" {
		return nil, "", apperr.NotFound("Belum ada bukti")
	}
	f, err := s.files.Open(res.EvidencePath)
	if err != nil {
		return nil, "", apperr.NotFound("Berkas bukti tidak ditemukan")
	}
	return f, res.EvidenceName, nil
}
