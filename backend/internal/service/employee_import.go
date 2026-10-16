package service

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/retechno/sikerja/backend/internal/apperr"
	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/internal/repository"
)

const (
	importSheet   = "Karyawan"
	maxImportRows = 1000
)

var importHeaders = []string{"NIK", "Nama", "Email", "Telepon", "Kode Departemen", "Jabatan", "Email Atasan", "Tanggal Masuk", "Role", "Password"}

type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportCredential struct {
	NIK          string `json:"nik"`
	Email        string `json:"email"`
	TempPassword string `json:"temp_password"`
}

type ImportResult struct {
	Created       int                `json:"created"`
	Errors        []ImportError      `json:"errors"`
	Credentials   []ImportCredential `json:"credentials"` // password sementara (hanya untuk baris tanpa kolom Password)
	PositionsMade int                `json:"positions_created"`
}

var dateLayouts = []string{"2006-01-02", "02/01/2006", "2/1/2006", "02-01-2006", "2 Jan 2006", "02 Jan 2006", "2 January 2006", "01-02-06", "1/2/06"}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil && n > 20000 && n < 80000 { // nomor seri Excel
		if t, err := excelize.ExcelDateToTime(n, false); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

var roleAlias = map[string]string{"karyawan": model.RoleEmployee, "manajer": model.RoleManager, "direktur": model.RoleDirector}

func splitRoles(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '/' }) {
		p = strings.ToLower(strings.TrimSpace(p))
		if a, ok := roleAlias[p]; ok {
			p = a
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type importRow struct {
	n         int // nomor baris di Excel
	in        EmployeeInput
	mgrEmail  string
	position  string
	password  string
	generated bool
}

// ImportEmployees membaca file .xlsx. Semua baris divalidasi lebih dulu; bila ada satu saja yang salah,
// tidak ada data yang disimpan dan seluruh kesalahan dilaporkan beserta nomor barisnya.
func (s *EmployeeService) ImportEmployees(a Actor, r io.Reader) (*ImportResult, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(r, 5<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 5<<20 {
		return nil, apperr.Validation("Berkas terlalu besar", map[string]string{"file": "Maksimal 5 MB"})
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.Validation("Berkas bukan Excel (.xlsx) yang valid", map[string]string{"file": "Gunakan template dari tombol Unduh template"})
	}
	defer f.Close()
	sheet := importSheet
	if idx, _ := f.GetSheetIndex(sheet); idx < 0 {
		sheet = f.GetSheetName(0)
	}
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil, apperr.Validation("Sheet kosong", map[string]string{"file": "Tidak ada data"})
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, h := range importHeaders[:3] {
		if _, ok := col[strings.ToLower(h)]; !ok {
			return nil, apperr.Validation("Kolom wajib tidak ditemukan: "+h, map[string]string{"file": "Gunakan template dari tombol Unduh template"})
		}
	}
	cell := func(row []string, name string) string {
		i, ok := col[strings.ToLower(name)]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	if len(rows)-1 > maxImportRows {
		return nil, apperr.Validation("Terlalu banyak baris", map[string]string{"file": fmt.Sprintf("Maksimal %d baris per berkas", maxImportRows)})
	}

	res := &ImportResult{Errors: []ImportError{}, Credentials: []ImportCredential{}}
	err = s.store.Tx(func(rp repository.Repos) error {
		depts, err := rp.Departments.List()
		if err != nil {
			return err
		}
		deptByKey := map[string]model.Department{}
		for _, d := range depts {
			deptByKey[strings.ToLower(d.Code)] = d
			deptByKey[strings.ToLower(d.Name)] = d
		}
		poss, err := rp.Positions.List(0)
		if err != nil {
			return err
		}
		posKey := func(deptID *uint64, name string) string {
			id := uint64(0)
			if deptID != nil {
				id = *deptID
			}
			return fmt.Sprintf("%d|%s", id, strings.ToLower(name))
		}
		posByKey := map[string]model.Position{}
		for _, p := range poss {
			posByKey[posKey(p.DepartmentID, p.Name)] = p
		}
		nikDB, err := rp.Employees.NIKMap()
		if err != nil {
			return err
		}
		emailDB, err := rp.Employees.EmailMap()
		if err != nil {
			return err
		}

		var parsed []importRow
		seenNIK, seenEmail := map[string]int{}, map[string]int{}
		addErr := func(row int, msg string) { res.Errors = append(res.Errors, ImportError{Row: row, Message: msg}) }

		for i, raw := range rows[1:] {
			n := i + 2
			blank := true
			for _, c := range raw {
				if strings.TrimSpace(c) != "" {
					blank = false
					break
				}
			}
			if blank {
				continue
			}
			ir := importRow{n: n, mgrEmail: strings.ToLower(cell(raw, "Email Atasan")), position: cell(raw, "Jabatan"), password: cell(raw, "Password")}
			ir.in = EmployeeInput{NIK: cell(raw, "NIK"), Name: cell(raw, "Nama"), Email: cell(raw, "Email"), Phone: cell(raw, "Telepon"),
				Roles: splitRoles(cell(raw, "Role")), Status: model.StatusActive}
			if d, ok := parseDate(cell(raw, "Tanggal Masuk")); ok {
				ir.in.JoinDate = d
			}
			if dk := cell(raw, "Kode Departemen"); dk != "" {
				if d, ok := deptByKey[strings.ToLower(dk)]; ok {
					id := d.ID
					ir.in.DepartmentID = &id
				} else {
					addErr(n, "Departemen \""+dk+"\" tidak ditemukan (isi dengan kode atau nama departemen)")
				}
			}
			fields, rerr := ir.in.validate(true)
			if rerr != nil {
				addErr(n, "Role tidak valid (gunakan karyawan, manajer, hr, direktur)")
			}
			keys := make([]string, 0, len(fields))
			for k := range fields {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				if k == "department_id" && cell(raw, "Kode Departemen") != "" {
					continue // sudah dilaporkan sebagai "departemen tidak ditemukan"
				}
				addErr(n, fields[k])
			}
			if ir.password != "" {
				if err := ValidatePassword(ir.password); err != nil {
					addErr(n, "Password minimal 8 karakter")
				}
			}
			if ir.in.NIK != "" {
				if _, dup := nikDB[ir.in.NIK]; dup {
					addErr(n, "NIK "+ir.in.NIK+" sudah terdaftar")
				}
				if prev, dup := seenNIK[ir.in.NIK]; dup {
					addErr(n, fmt.Sprintf("NIK %s kembar dengan baris %d", ir.in.NIK, prev))
				}
				seenNIK[ir.in.NIK] = n
			}
			if ir.in.Email != "" {
				if _, dup := emailDB[ir.in.Email]; dup {
					addErr(n, "Email "+ir.in.Email+" sudah terdaftar")
				}
				if prev, dup := seenEmail[ir.in.Email]; dup {
					addErr(n, fmt.Sprintf("Email %s kembar dengan baris %d", ir.in.Email, prev))
				}
				seenEmail[ir.in.Email] = n
			}
			parsed = append(parsed, ir)
		}
		if len(parsed) == 0 && len(res.Errors) == 0 {
			addErr(0, "Tidak ada baris data")
		}
		// atasan: harus ada di database atau di berkas ini
		for _, ir := range parsed {
			if ir.mgrEmail == "" {
				continue
			}
			_, inDB := emailDB[ir.mgrEmail]
			_, inFile := seenEmail[ir.mgrEmail]
			if !inDB && !inFile {
				addErr(ir.n, "Email atasan \""+ir.mgrEmail+"\" tidak ditemukan")
			}
			if ir.mgrEmail == ir.in.Email {
				addErr(ir.n, "Atasan tidak boleh diri sendiri")
			}
		}
		if len(res.Errors) > 0 {
			return errImportInvalid
		}

		created := map[string]uint64{} // email → employee id
		for i := range parsed {
			ir := &parsed[i]
			if ir.position != "" {
				k := posKey(ir.in.DepartmentID, ir.position)
				p, ok := posByKey[k]
				if !ok {
					p = model.Position{Name: ir.position, DepartmentID: ir.in.DepartmentID}
					if err := rp.Positions.Create(&p); err != nil {
						return err
					}
					posByKey[k] = p
					res.PositionsMade++
				}
				id := p.ID
				ir.in.PositionID = &id
			}
			pw := ir.password
			if pw == "" {
				pw = randomPassword(10)
				ir.generated = true
			}
			e, err := createEmployee(rp, ir.in, pw)
			if err != nil {
				return err
			}
			created[ir.in.Email] = e.ID
			if ir.generated {
				res.Credentials = append(res.Credentials, ImportCredential{NIK: ir.in.NIK, Email: ir.in.Email, TempPassword: pw})
			}
		}
		// atasan diselesaikan setelah semua baris dibuat (boleh merujuk baris lain)
		for _, ir := range parsed {
			if ir.mgrEmail == "" {
				continue
			}
			mid, ok := created[ir.mgrEmail]
			if !ok {
				mid = emailDB[ir.mgrEmail]
			}
			e, err := rp.Employees.ByID(created[ir.in.Email])
			if err != nil {
				return err
			}
			e.ManagerID = &mid
			if err := rp.Employees.Update(e); err != nil {
				return err
			}
		}
		res.Created = len(parsed)
		return logActivity(rp, a, "employee.import", "employee", 0, nil, map[string]any{"created": res.Created})
	})
	if err == errImportInvalid {
		res.Created = 0
		return res, nil
	}
	if err != nil {
		return nil, err
	}
	s.cache.Bump(bgCtx())
	return res, nil
}

var errImportInvalid = fmt.Errorf("import tidak valid")

// ImportTemplate membuat berkas template .xlsx: sheet "Karyawan" (header + contoh) dan "Petunjuk".
func (s *EmployeeService) ImportTemplate(a Actor) ([]byte, error) {
	if err := requireHR(a); err != nil {
		return nil, err
	}
	depts, err := s.store.R().Departments.List()
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()
	idx, _ := f.NewSheet(importSheet)
	_ = f.DeleteSheet("Sheet1")
	f.SetActiveSheet(idx)

	head := make([]any, len(importHeaders))
	for i, h := range importHeaders {
		head[i] = h
	}
	_ = f.SetSheetRow(importSheet, "A1", &head)
	code := "ITS"
	if len(depts) > 0 {
		code = depts[0].Code
	}
	_ = f.SetSheetRow(importSheet, "A2", &[]any{"2026-0101", "Contoh Nama", "contoh.nama@perusahaan.co.id", "081234567890", code, "IT Support Engineer", "atasan@perusahaan.co.id", "2026-01-15", "karyawan", ""})
	hs, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"0E5A52"}}})
	_ = f.SetCellStyle(importSheet, "A1", "J1", hs)
	_ = f.SetColWidth(importSheet, "A", "J", 22)
	// kolom tanggal & NIK sebagai teks agar tidak diubah Excel
	txt, _ := f.NewStyle(&excelize.Style{NumFmt: 49})
	_ = f.SetColStyle(importSheet, "A,H", txt)

	help := "Petunjuk"
	_, _ = f.NewSheet(help)
	lines := [][]any{
		{"Petunjuk pengisian"},
		{"1. Isi sheet \"Karyawan\" mulai baris 2; hapus baris contoh."},
		{"2. Kolom wajib: NIK, Nama, Email, Kode Departemen (kecuali role hr/direktur), Tanggal Masuk."},
		{"3. Tanggal Masuk: YYYY-MM-DD (mis. 2026-01-15) atau DD/MM/YYYY."},
		{"4. Role: karyawan, manajer, hr, direktur (boleh lebih dari satu, pisahkan dengan koma). Kosong = karyawan."},
		{"5. Email Atasan: email karyawan lain (boleh yang ada di berkas ini)."},
		{"6. Jabatan: ditulis bebas; jika belum ada akan dibuat otomatis di departemen tersebut."},
		{"7. Password: opsional (min. 8 karakter). Jika kosong, sistem membuat password sementara dan menampilkannya sekali setelah impor."},
		{"8. Jika ada satu baris yang salah, tidak ada data yang diimpor; perbaiki lalu unggah ulang."},
		{},
		{"Kode Departemen", "Nama"},
	}
	for i, l := range lines {
		ln := l
		_ = f.SetSheetRow(help, fmt.Sprintf("A%d", i+1), &ln)
	}
	for i, d := range depts {
		_ = f.SetSheetRow(help, fmt.Sprintf("A%d", len(lines)+1+i), &[]any{d.Code, d.Name})
	}
	_ = f.SetColWidth(help, "A", "B", 40)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
