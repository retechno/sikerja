<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, master, reports } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtScore, gradeClass } from '@/lib/format'
import { usePeriodStore } from '@/stores/period'
import { useToastStore } from '@/stores/toast'

// H-15 — Laporan: filter periode/departemen/predikat, tabel skor, ekspor Excel & PDF.
const period = usePeriodStore()
const toast = useToastStore()

const deptId = ref<number | ''>('')
const grade = ref('')
const search = ref('')
const page = ref(1)
const perPage = 15
const desc = ref(true)
const exporting = ref<'' | 'xlsx' | 'pdf'>('')

const { data, loading, error, reload } = useLoader(async () => {
  await period.load()
  const [report, depts] = await Promise.all([
    reports.scores({ period_id: period.selectedId ?? undefined, department_id: deptId.value || undefined }),
    master.departments(),
  ])
  return { report, depts }
}, [() => period.selectedId, deptId])
watch([grade, search, desc, () => period.selectedId, deptId], () => (page.value = 1))

const GRADES = ['Istimewa', 'Baik', 'Cukup', 'Perlu Perbaikan', 'Kurang']

const filtered = computed(() => {
  const rows = (data.value?.report.rows ?? []).filter((r) => {
    if (grade.value && r.grade !== grade.value) return false
    const q = search.value.trim().toLowerCase()
    return !q || r.name.toLowerCase().includes(q) || r.nik.toLowerCase().includes(q)
  })
  const val = (x: string | null) => (x == null ? -1 : Number(x))
  return [...rows].sort((a, b) =>
    desc.value ? val(b.final_score) - val(a.final_score) : val(a.final_score) - val(b.final_score),
  )
})

const scored = computed(() => filtered.value.filter((r) => r.final_score != null))
const avg = (xs: (string | null)[]) => {
  const n = xs.filter((x): x is string => x != null).map(Number)
  return n.length ? n.reduce((a, b) => a + b, 0) / n.length : null
}
const stats = computed(() => {
  const f = scored.value.map((r) => Number(r.final_score))
  return {
    count: scored.value.length,
    avgFinal: avg(scored.value.map((r) => r.final_score)),
    avgKpi: avg(scored.value.map((r) => r.kpi_score)),
    avgBeh: avg(scored.value.map((r) => r.behavior_score)),
    min: f.length ? Math.min(...f) : null,
    max: f.length ? Math.max(...f) : null,
  }
})

const pages = computed(() => Math.max(1, Math.ceil(filtered.value.length / perPage)))
const slice = computed(() => filtered.value.slice((page.value - 1) * perPage, page.value * perPage))

async function doExport(format: 'xlsx' | 'pdf') {
  exporting.value = format
  try {
    await reports.export(format, {
      period_id: period.selectedId ?? undefined,
      department_id: deptId.value || undefined,
    })
    toast.success(`Laporan ${format.toUpperCase()} diunduh.`)
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal mengekspor')
  } finally {
    exporting.value = ''
  }
}
</script>

<template>
  <div class="flex flex-wrap items-end gap-3">
    <label class="field"
      >Departemen
      <select v-model="deptId" class="input" data-testid="dept-filter">
        <option value="">Semua departemen</option>
        <option v-for="d in data?.depts ?? []" :key="d.id" :value="d.id">{{ d.name }}</option>
      </select>
    </label>
    <label class="field"
      >Predikat
      <select v-model="grade" class="input" data-testid="grade-filter">
        <option value="">Semua predikat</option>
        <option v-for="g in GRADES" :key="g" :value="g">{{ g }}</option>
      </select>
    </label>
    <label class="field min-w-[200px] flex-1"
      >Cari
      <input v-model="search" type="search" class="input" placeholder="Nama atau NIK" data-testid="search" />
    </label>
    <button
      type="button"
      class="btn"
      :disabled="!!exporting || !data"
      data-testid="export-xlsx"
      @click="doExport('xlsx')"
    >
      <Icon name="download" />{{ exporting === 'xlsx' ? 'Mengekspor…' : 'Excel' }}
    </button>
    <button
      type="button"
      class="btn pri"
      :disabled="!!exporting || !data"
      data-testid="export-pdf"
      @click="doExport('pdf')"
    >
      <Icon name="download" />{{ exporting === 'pdf' ? 'Mengekspor…' : 'PDF' }}
    </button>
  </div>

  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <template v-if="data">
      <div class="grid grid-cols-[repeat(auto-fit,minmax(210px,1fr))] gap-4">
        <div class="card">
          <div class="k">Karyawan dinilai</div>
          <div class="v" data-testid="stat-count">{{ stats.count }}</div>
          <div class="d">dari {{ filtered.length }} karyawan</div>
        </div>
        <div class="card">
          <div class="k">Rata-rata skor akhir</div>
          <div class="v" data-testid="stat-avg">{{ fmtScore(stats.avgFinal) }}</div>
        </div>
        <div class="card">
          <div class="k">Rata-rata KPI / perilaku</div>
          <div class="v !text-2xl">{{ fmtScore(stats.avgKpi) }} / {{ fmtScore(stats.avgBeh) }}</div>
        </div>
        <div class="card">
          <div class="k">Rentang skor</div>
          <div class="v !text-2xl">{{ fmtScore(stats.min) }} – {{ fmtScore(stats.max) }}</div>
        </div>
      </div>

      <section class="card !p-2 pb-1">
        <div class="px-3.5 pb-3 pt-3">
          <h2>Skor {{ data.report.period.name }} · {{ data.report.department_name }}</h2>
          <p class="hint">
            Skor akhir = {{ Number(data.report.period.kpi_weight) }}% KPI +
            {{ Number(data.report.period.behavior_weight) }}% perilaku. Skor tampil setelah atasan mengirim
            penilaian.
          </p>
        </div>
        <p v-if="!filtered.length" class="m-0 px-3.5 pb-4 text-ink-soft">
          Tidak ada data yang cocok dengan filter.
        </p>
        <div v-else class="overflow-x-auto">
          <table class="tbl" data-testid="report-table">
            <thead>
              <tr>
                <th>#</th>
                <th>Karyawan</th>
                <th>Departemen</th>
                <th class="num">Skor KPI</th>
                <th class="num">Perilaku</th>
                <th class="num">
                  <button type="button" class="font-semibold uppercase tracking-wider" @click="desc = !desc">
                    Skor akhir {{ desc ? '↓' : '↑' }}
                  </button>
                </th>
                <th>Predikat</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(r, i) in slice" :key="r.employee_id">
                <td>{{ (page - 1) * perPage + i + 1 }}</td>
                <td>
                  <b>{{ r.name }}</b>
                  <div class="muted text-xs">{{ r.position || r.nik }}</div>
                </td>
                <td>{{ r.department }}</td>
                <td class="num">{{ fmtScore(r.kpi_score) }}</td>
                <td class="num">{{ fmtScore(r.behavior_score) }}</td>
                <td class="num">
                  <b>{{ fmtScore(r.final_score) }}</b>
                </td>
                <td>
                  <span v-if="r.grade" class="badge" :class="gradeClass(r.grade)">{{ r.grade }}</span
                  ><span v-else class="muted">—</span>
                </td>
                <td>
                  <span class="badge b-mute">{{ r.status }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3 px-3.5 py-3 text-[13px] text-ink-soft">
          <span
            >Menampilkan {{ filtered.length ? (page - 1) * perPage + 1 : 0 }}–{{
              Math.min(page * perPage, filtered.length)
            }}
            dari {{ filtered.length }}</span
          >
          <div class="flex items-center gap-2">
            <button type="button" class="btn sm" :disabled="page <= 1" @click="page--">Sebelumnya</button>
            <span>Hal. {{ page }} / {{ pages }}</span>
            <button type="button" class="btn sm" :disabled="page >= pages" @click="page++">Berikutnya</button>
          </div>
        </div>
      </section>
    </template>
  </PageState>
</template>
