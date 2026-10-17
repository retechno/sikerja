package service

// DefaultIndicator adalah satu baris "KPI default per departemen" di SPEC.
// Target/Weight berupa string agar presisi desimal terjaga.
type DefaultIndicator struct {
	Name      string
	Unit      string
	Target    string
	Weight    string // persen
	Direction string
}

// DepartmentDef adalah departemen awal untuk seed.
type DepartmentDef struct {
	Code      string
	Name      string
	Positions []string
}

var DefaultDepartments = []DepartmentDef{
	{"ITS", "IT Support", []string{"Kepala IT Support", "IT Support Engineer"}},
	{"CS", "Customer Service", []string{"Kepala Customer Service", "Customer Service Officer"}},
	{"SM", "Sales & Marketing", []string{"Kepala Sales & Marketing", "Account Executive"}},
	{"DEV", "Developer", []string{"Kepala Developer", "Software Developer"}},
}

// DefaultKpiTemplates persis mengikuti tabel "KPI default per departemen" di SPEC.
// Kunci = nama departemen.
var DefaultKpiTemplates = map[string][]DefaultIndicator{
	"IT Support": {
		{"Tiket terselesaikan", "tiket", "120", "25", DirectionHigher},
		{"Tiket selesai dalam SLA", "%", "95", "25", DirectionHigher},
		{"Rata-rata waktu respons pertama", "menit", "15", "15", DirectionLower},
		{"Instalasi / training PMS di hotel klien", "kegiatan", "4", "20", DirectionHigher},
		{"Rating kepuasan klien", "dari 5", "4.5", "15", DirectionHigher},
	},
	"Customer Service": {
		{"Rata-rata waktu respons chat/telepon", "menit", "5", "25", DirectionLower},
		{"Kasus selesai tanpa eskalasi", "%", "80", "25", DirectionHigher},
		{"Skor kepuasan (CSAT)", "%", "90", "30", DirectionHigher},
		{"Jumlah kasus ditangani", "kasus", "300", "20", DirectionHigher},
	},
	"Sales & Marketing": {
		{"Pendapatan tercapai vs target", "%", "100", "35", DirectionHigher},
		{"Hotel klien baru", "hotel", "3", "20", DirectionHigher},
		{"Demo PMS dilakukan", "demo", "12", "15", DirectionHigher},
		{"Konversi lead ke deal", "%", "20", "15", DirectionHigher},
		{"Perpanjangan kontrak klien", "%", "90", "15", DirectionHigher},
	},
	"Developer": {
		{"Task selesai tepat waktu", "%", "90", "30", DirectionHigher},
		{"Story point diselesaikan", "poin", "40", "20", DirectionHigher},
		{"Bug setelah rilis", "bug", "3", "25", DirectionLower},
		{"Rilis sesuai jadwal", "%", "100", "15", DirectionHigher},
		{"Code review dikerjakan", "review", "15", "10", DirectionHigher},
	},
}

// DefaultAspects = 5 aspek perilaku yang sama untuk semua departemen.
var DefaultAspects = []struct{ Name, Description string }{
	{"Disiplin", "Tepat waktu, mematuhi SLA dan prosedur"},
	{"Kerja sama tim", "Membantu rekan, berbagi pengetahuan"},
	{"Inisiatif", "Mengusulkan perbaikan tanpa diminta"},
	{"Komunikasi", "Jelas kepada klien hotel dan tim"},
	{"Tanggung jawab", "Menuntaskan pekerjaan sampai tuntas dan berkualitas"},
}
