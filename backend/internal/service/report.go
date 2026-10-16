package service

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

type ReportService struct {
	store repository.Store
}

func NewReportService(store repository.Store) *ReportService { return &ReportService{store: store} }

type ReportRow struct {
	EmployeeID    uint64           `json:"employee_id"`
	NIK           string           `json:"nik"`
	Name          string           `json:"name"`
	Department    string           `json:"department"`
	Position      string           `json:"position"`
	KpiScore      *decimal.Decimal `json:"kpi_score"`
	BehaviorScore *decimal.Decimal `json:"behavior_score"`
	FinalScore    *decimal.Decimal `json:"final_score"`
	Grade         string           `json:"grade"`
	Status        string           `json:"status"`
}

type ReportSummary struct {
	Total  int              `json:"total"`
	Scored int              `json:"scored"`
	Avg    *decimal.Decimal `json:"avg"`
}

type ReportView struct {
	Period         *model.AppraisalPeriod `json:"period"`
	DepartmentID   uint64                 `json:"department_id"`
	DepartmentName string                 `json:"department_name"`
	Rows           []ReportRow            `json:"rows"`
	Summary        ReportSummary          `json:"summary"`
}

var statusLabel = map[string]string{
	"":                         "Belum dinilai",
	model.AppraisalDraft:       "Draf",
	model.AppraisalSelfDone:    "Self-assessment selesai",
	model.AppraisalManagerDone: "Dinilai atasan",
	model.AppraisalFinal:       "Final",
}

func canReport(a Actor) error {
	if !a.HasAny(model.RoleHR, model.RoleDirector) {
		return apperr.Forbidden("Laporan hanya untuk HR dan Direktur")
	}
	return nil
}

func nullPtr(n decimal.NullDecimal) *decimal.Decimal {
	if !n.Valid {
		return nil
	}
	v := n.Decimal
	return &v
}

// Build menyusun tabel skor satu periode (opsional satu departemen) dengan satu query.
func (s *ReportService) Build(a Actor, periodID, deptID uint64) (*ReportView, error) {
	if err := canReport(a); err != nil {
		return nil, err
	}
	r := s.store.R()
	p, err := resolvePeriod(r, periodID)
	if err != nil {
		return nil, err
	}
	v := &ReportView{Period: p, DepartmentID: deptID, DepartmentName: "Semua departemen", Rows: []ReportRow{}}
	if deptID != 0 {
		d, err := r.Departments.ByID(deptID)
		if err != nil {
			return nil, apperr.NotFound("Departemen tidak ditemukan")
		}
		v.DepartmentName = d.Name
	}
	rows, err := r.Reports.ScoresForPeriod(p.ID, deptID, nil)
	if err != nil {
		return nil, err
	}
	sum := decimal.Zero
	for _, x := range rows {
		st := ""
		if x.Status != nil {
			st = *x.Status
		}
		row := ReportRow{EmployeeID: x.EmployeeID, NIK: x.NIK, Name: x.EmployeeName, Department: x.DepartmentName, Position: x.PositionName,
			Status: statusLabel[st]}
		// skor baru ditampilkan setelah atasan mengirim penilaian (manager_done / final)
		if st == model.AppraisalManagerDone || st == model.AppraisalFinal {
			row.KpiScore, row.BehaviorScore, row.FinalScore = nullPtr(x.KpiScore), nullPtr(x.BehaviorScore), nullPtr(x.FinalScore)
			if x.Grade != nil {
				row.Grade = *x.Grade
			}
		}
		if row.FinalScore != nil {
			v.Summary.Scored++
			sum = sum.Add(*row.FinalScore)
		}
		v.Rows = append(v.Rows, row)
	}
	v.Summary.Total = len(v.Rows)
	if v.Summary.Scored > 0 {
		avg := sum.Div(decimal.NewFromInt(int64(v.Summary.Scored))).Round(2)
		v.Summary.Avg = &avg
	}
	return v, nil
}

func fmtScore(d *decimal.Decimal) string {
	if d == nil {
		return "-"
	}
	return FormatScore(*d)
}

func fileStem(v *ReportView) string {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				return r
			}
			return '-'
		}, s)
	}
	return "laporan-kinerja-" + clean(v.Period.Name) + "-" + clean(v.DepartmentName)
}

// ExportXLSX membuat laporan Excel.
func (s *ReportService) ExportXLSX(a Actor, periodID, deptID uint64) ([]byte, string, error) {
	v, err := s.Build(a, periodID, deptID)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	defer f.Close()
	const sh = "Laporan"
	_ = f.SetSheetName("Sheet1", sh)
	_ = f.SetCellValue(sh, "A1", "Laporan Kinerja — "+v.Period.Name)
	_ = f.SetCellValue(sh, "A2", v.DepartmentName+" · dicetak "+time.Now().Format("02 Jan 2006"))
	title, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	_ = f.SetCellStyle(sh, "A1", "A1", title)

	head := []any{"No", "NIK", "Nama", "Departemen", "Jabatan", "Skor KPI", "Skor Perilaku", "Skor Akhir", "Predikat", "Status"}
	_ = f.SetSheetRow(sh, "A4", &head)
	hs, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"0E5A52"}}})
	_ = f.SetCellStyle(sh, "A4", "J4", hs)
	num, _ := f.NewStyle(&excelize.Style{NumFmt: 2}) // 0.00
	numOrDash := func(d *decimal.Decimal) any {
		if d == nil {
			return "-"
		}
		fl, _ := d.Round(2).Float64()
		return fl
	}
	for i, r := range v.Rows {
		line := []any{i + 1, r.NIK, r.Name, r.Department, r.Position, numOrDash(r.KpiScore), numOrDash(r.BehaviorScore), numOrDash(r.FinalScore), orDash(r.Grade), r.Status}
		_ = f.SetSheetRow(sh, fmt.Sprintf("A%d", 5+i), &line)
	}
	if n := len(v.Rows); n > 0 {
		_ = f.SetCellStyle(sh, "F5", fmt.Sprintf("H%d", 4+n), num)
	}
	last := 5 + len(v.Rows) + 1
	_ = f.SetCellValue(sh, fmt.Sprintf("C%d", last), "Rata-rata skor akhir")
	if v.Summary.Avg != nil {
		fl, _ := v.Summary.Avg.Float64()
		_ = f.SetCellValue(sh, fmt.Sprintf("H%d", last), fl)
		_ = f.SetCellStyle(sh, fmt.Sprintf("H%d", last), fmt.Sprintf("H%d", last), num)
	}
	_ = f.SetColWidth(sh, "A", "A", 5)
	_ = f.SetColWidth(sh, "B", "B", 14)
	_ = f.SetColWidth(sh, "C", "E", 26)
	_ = f.SetColWidth(sh, "F", "I", 15)
	_ = f.SetColWidth(sh, "J", "J", 24)
	_ = f.SetPanes(sh, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), fileStem(v) + ".xlsx", nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ExportPDF membuat laporan PDF (lanskap).
func (s *ReportService) ExportPDF(a Actor, periodID, deptID uint64) ([]byte, string, error) {
	v, err := s.Build(a, periodID, deptID)
	if err != nil {
		return nil, "", err
	}
	cfg := config.NewBuilder().
		WithOrientation(orientation.Horizontal).
		WithLeftMargin(10).WithRightMargin(10).WithTopMargin(10).WithBottomMargin(12).
		WithPageNumber().
		WithTitle("Laporan Kinerja "+v.Period.Name, false).
		Build()
	m := maroto.New(cfg)

	teal := &props.Color{Red: 14, Green: 90, Blue: 82}
	white := &props.Color{Red: 255, Green: 255, Blue: 255}
	zebra := &props.Color{Red: 244, Green: 245, Blue: 241}

	cell := func(size int, v string, st fontstyle.Type, al align.Type, color *props.Color) core.Col {
		return col.New(size).Add(text.New(v, props.Text{Size: 9, Style: st, Align: al, Top: 1.5, Left: 1, Right: 1, Color: color}))
	}
	m.AddRows(
		text.NewRow(9, "Laporan Kinerja — "+v.Period.Name, props.Text{Size: 15, Style: fontstyle.Bold}),
		text.NewRow(6, fmt.Sprintf("%s · %d karyawan · dicetak %s", v.DepartmentName, v.Summary.Total, time.Now().Format("02 Jan 2006")),
			props.Text{Size: 9, Color: &props.Color{Red: 91, Green: 102, Blue: 98}}),
		row.New(4),
	)
	m.RegisterHeader(row.New(8).WithStyle(&props.Cell{BackgroundColor: teal}).Add(
		cell(1, "No", fontstyle.Bold, align.Left, white),
		cell(3, "Nama", fontstyle.Bold, align.Left, white),
		cell(2, "Departemen", fontstyle.Bold, align.Left, white),
		cell(1, "KPI", fontstyle.Bold, align.Right, white),
		cell(1, "Perilaku", fontstyle.Bold, align.Right, white),
		cell(1, "Akhir", fontstyle.Bold, align.Right, white),
		cell(2, "Predikat", fontstyle.Bold, align.Left, white),
		cell(1, "Status", fontstyle.Bold, align.Left, white),
	))
	for i, r := range v.Rows {
		rw := row.New(7).Add(
			cell(1, fmt.Sprint(i+1), fontstyle.Normal, align.Left, nil),
			cell(3, r.Name, fontstyle.Normal, align.Left, nil),
			cell(2, r.Department, fontstyle.Normal, align.Left, nil),
			cell(1, fmtScore(r.KpiScore), fontstyle.Normal, align.Right, nil),
			cell(1, fmtScore(r.BehaviorScore), fontstyle.Normal, align.Right, nil),
			cell(1, fmtScore(r.FinalScore), fontstyle.Bold, align.Right, nil),
			cell(2, orDash(r.Grade), fontstyle.Normal, align.Left, nil),
			cell(1, shortStatus(r.Status), fontstyle.Normal, align.Left, nil),
		)
		if i%2 == 1 {
			rw = rw.WithStyle(&props.Cell{BackgroundColor: zebra})
		}
		m.AddRows(rw)
	}
	avg := "-"
	if v.Summary.Avg != nil {
		avg = FormatScore(*v.Summary.Avg)
	}
	m.AddRows(row.New(3), text.NewRow(7, fmt.Sprintf("Rata-rata skor akhir: %s (dari %d karyawan yang sudah dinilai)", avg, v.Summary.Scored),
		props.Text{Size: 10, Style: fontstyle.Bold}))

	doc, err := m.Generate()
	if err != nil {
		return nil, "", err
	}
	return doc.GetBytes(), fileStem(v) + ".pdf", nil
}

func shortStatus(s string) string {
	switch s {
	case "Self-assessment selesai":
		return "Self"
	case "Dinilai atasan":
		return "Atasan"
	case "Belum dinilai":
		return "Belum"
	}
	return s
}
