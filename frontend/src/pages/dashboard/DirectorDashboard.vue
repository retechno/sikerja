<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import BarChart from '@/components/charts/BarChart.vue'
import LineChart from '@/components/charts/LineChart.vue'
import PendingApprovals from '@/components/leave/PendingApprovals.vue'
import { dashboard, leave } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtScore, gradeClass } from '@/lib/format'
import { usePeriodStore } from '@/stores/period'

// H-04 — Dashboard Direktur
const period = usePeriodStore()
const { data, loading, error, reload } = useLoader(async () => {
  await period.load()
  return dashboard.director(period.selectedId ?? undefined)
}, [() => period.selectedId])

// pengajuan izin/cuti yang menunggu keputusan Direktur/HR
const leaveSummary = useLoader(() => leave.summary())

const GRADE_COLORS: Record<string, string> = {
  Istimewa: '#0E5A52',
  Baik: '#4C9A8C',
  Cukup: '#E8A33D',
  'Perlu Perbaikan': '#D9763B',
  Kurang: '#B3261E',
}

const delta = computed(() => (data.value?.company_delta != null ? Number(data.value.company_delta) : null))
const istimewa = computed(() => data.value?.grades.find((g) => g.grade === 'Istimewa')?.count ?? 0)
const pct = computed(() =>
  data.value && data.value.scored ? Math.round((istimewa.value / data.value.scored) * 100) : 0,
)
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="data">
      <div>
        <h2 class="m-0 text-xl font-bold">Kinerja perusahaan</h2>
        <p class="hint">
          {{ data.scored }} dari {{ data.employees }} karyawan sudah memiliki skor · {{ data.period.name }}
        </p>
      </div>

      <div class="grid grid-cols-[repeat(auto-fit,minmax(230px,1fr))] gap-4">
        <div class="card" data-testid="company-card">
          <div class="k">Rata-rata perusahaan</div>
          <div class="v">{{ fmtScore(data.company_avg) }}</div>
          <div class="d">
            <template v-if="delta !== null"
              ><span :class="delta >= 0 ? 'up' : 'down'"
                >{{ delta >= 0 ? '+' : '−' }}{{ fmtScore(Math.abs(delta)) }}</span
              >
              dari periode sebelumnya</template
            >
            <template v-if="data.company_grade"> · predikat {{ data.company_grade }}</template>
            <template v-if="delta === null && !data.company_grade">belum ada skor</template>
          </div>
        </div>
        <div class="card">
          <div class="k">Departemen terbaik</div>
          <div class="v !text-2xl">{{ data.best_department?.name ?? '—' }}</div>
          <div class="d">
            <template v-if="data.best_department"
              >rata-rata {{ fmtScore(data.best_department.avg) }}</template
            >
          </div>
        </div>
        <div class="card">
          <div class="k">Predikat Istimewa</div>
          <div class="v">{{ istimewa }} <span class="text-base font-semibold text-ink-soft">orang</span></div>
          <div class="d">{{ pct }}% dari yang dinilai</div>
        </div>
        <div class="card" data-testid="attention-card">
          <div class="k">Perlu perhatian (&lt; 70)</div>
          <div class="v">
            {{ data.attention }} <span class="text-base font-semibold text-ink-soft">orang</span>
          </div>
          <div class="d"><RouterLink to="/laporan">Lihat laporan →</RouterLink></div>
        </div>
      </div>

      <PendingApprovals
        v-if="leaveSummary.data.value && leaveSummary.data.value.pending_count > 0"
        :items="leaveSummary.data.value.queue"
        :total="leaveSummary.data.value.pending_count"
      />

      <div class="flex flex-wrap items-start gap-6">
        <section class="card min-w-0 flex-[1_1_420px]">
          <h2>Skor rata-rata per departemen</h2>
          <p class="hint mb-3">{{ data.period.name }}, skala 0–100</p>
          <p v-if="!data.departments.length" class="muted m-0">Belum ada skor pada periode ini.</p>
          <BarChart
            v-else
            horizontal
            :labels="data.departments.map((d) => d.name)"
            :values="data.departments.map((d) => Number(d.avg))"
            :highlight="0"
            :height="Math.max(160, data.departments.length * 52)"
            description="Skor rata-rata per departemen"
          />
        </section>
        <section class="card min-w-0 flex-[1_1_420px]">
          <h2>Tren skor perusahaan</h2>
          <p class="hint mb-3">{{ data.trend.length }} periode terakhir</p>
          <LineChart
            :labels="data.trend.map((t) => t.name.split(' ')[0] ?? t.name)"
            :values="data.trend.map((t) => (t.avg ? Number(t.avg) : null))"
            description="Tren skor rata-rata perusahaan"
          />
        </section>
      </div>

      <div class="flex flex-wrap items-start gap-6">
        <section class="card min-w-0 flex-[1_1_420px]">
          <h2>Sebaran predikat</h2>
          <p class="hint mb-3">{{ data.scored }} karyawan, {{ data.period.name }}</p>
          <BarChart
            :labels="data.grades.map((g) => g.grade)"
            :values="data.grades.map((g) => g.count)"
            :colors="data.grades.map((g) => GRADE_COLORS[g.grade] ?? '#999')"
            :max="Math.max(5, ...data.grades.map((g) => g.count))"
            description="Jumlah karyawan per predikat"
          />
          <ul class="m-0 mt-3 flex list-none flex-wrap gap-x-4 gap-y-1 p-0 text-xs text-ink-soft">
            <li v-for="g in data.grades" :key="g.grade">
              <b class="text-ink">{{ g.count }}</b> {{ g.grade }}
            </li>
          </ul>
        </section>

        <section class="card min-w-0 flex-[1_1_340px]" data-testid="top-card">
          <h2>5 skor tertinggi</h2>
          <p v-if="!data.top.length" class="muted mb-0 mt-2">Belum ada skor.</p>
          <div
            v-for="s in data.top"
            :key="s.employee_id"
            class="flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0"
          >
            <div class="min-w-0 flex-1">
              <b>{{ s.name }}</b>
              <div class="muted text-xs">{{ s.department }}</div>
            </div>
            <b class="tabular-nums">{{ fmtScore(s.score) }}</b>
            <span class="badge" :class="gradeClass(s.grade)">{{ s.grade }}</span>
          </div>
        </section>

        <section class="card min-w-0 flex-[1_1_340px]" data-testid="bottom-card">
          <div class="flex items-start justify-between gap-3">
            <h2>5 skor terendah</h2>
            <RouterLink to="/laporan" class="text-[13px] font-semibold">Buka laporan</RouterLink>
          </div>
          <p v-if="!data.bottom.length" class="muted mb-0 mt-2">Belum ada skor.</p>
          <div
            v-for="s in data.bottom"
            :key="s.employee_id"
            class="flex items-center gap-3 border-b border-line-soft py-2.5 last:border-b-0"
          >
            <div class="min-w-0 flex-1">
              <b>{{ s.name }}</b>
              <div class="muted text-xs">{{ s.department }}</div>
            </div>
            <b class="tabular-nums">{{ fmtScore(s.score) }}</b>
            <span class="badge" :class="gradeClass(s.grade)">{{ s.grade }}</span>
          </div>
        </section>
      </div>
    </template>
  </PageState>
</template>
