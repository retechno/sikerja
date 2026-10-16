<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import BarChart from '@/components/charts/BarChart.vue'
import { dashboard } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtLongDate, fmtNum, fmtScore, gradeClass, APPRAISAL_STATUS } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { usePeriodStore } from '@/stores/period'

// H-02 — Dashboard Karyawan
const auth = useAuthStore()
const period = usePeriodStore()
const { data, loading, error, reload } = useLoader(async () => {
  await period.load()
  return dashboard.employee(period.selectedId ?? undefined)
}, [() => period.selectedId])

const firstName = computed(() => auth.user?.name.split(' ')[0] ?? '')
const rows = computed(() => data.value?.sheet.rows ?? [])
const delta = computed(() => (data.value?.kpi_delta != null ? Number(data.value.kpi_delta) : null))
const history = computed(() => data.value?.history ?? [])

function unitText(v: string, unit: string): string {
  if (unit === '%') return `${fmtNum(v)}%`
  if (unit === 'dari 5' || !unit) return fmtNum(v)
  return `${fmtNum(v)} ${unit}`
}
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="data">
      <div>
        <h2 class="m-0 text-xl font-bold">Halo, {{ firstName }}</h2>
        <p class="hint">{{ fmtLongDate(new Date()) }} · periode {{ data.period.name }}</p>
      </div>

      <div class="grid grid-cols-[repeat(auto-fit,minmax(230px,1fr))] gap-4">
        <div class="card" data-testid="kpi-card">
          <div class="k">Skor KPI sementara · {{ data.period.name }}</div>
          <div class="v">{{ fmtScore(data.kpi_score) }}</div>
          <div class="d">
            <template v-if="delta !== null">
              <span :class="delta >= 0 ? 'up' : 'down'"
                >{{ delta >= 0 ? '+' : '−' }}{{ fmtScore(Math.abs(delta)) }}</span
              >
              dibanding {{ data.prev_period_name }}
            </template>
            <template v-else>dari {{ data.sheet.total }} indikator berbobot</template>
          </div>
        </div>

        <div class="card" data-testid="final-card">
          <template v-if="data.final_score">
            <div class="k">Skor akhir · {{ data.period.name }}</div>
            <div class="v">
              {{ fmtScore(data.final_score) }}
              <span class="badge align-middle" :class="gradeClass(data.grade)">{{ data.grade }}</span>
            </div>
            <div class="d">penilaian sudah final</div>
          </template>
          <template v-else-if="data.latest_final">
            <div class="k">Skor akhir · {{ data.latest_final.period_name }}</div>
            <div class="v">
              {{ fmtScore(data.latest_final.score) }}
              <span class="badge align-middle" :class="gradeClass(data.latest_final.grade)">{{
                data.latest_final.grade
              }}</span>
            </div>
            <div class="d">
              <template v-if="data.latest_final.next"
                >{{ fmtScore(data.latest_final.next.gap) }} lagi menuju
                {{ data.latest_final.next.grade }}</template
              >
              <template v-else>predikat tertinggi</template>
            </div>
          </template>
          <template v-else>
            <div class="k">Skor akhir</div>
            <div class="v">—</div>
            <div class="d">belum ada penilaian final</div>
          </template>
        </div>

        <div class="card">
          <div class="k">Batas input realisasi</div>
          <template v-if="data.period.status === 'active'">
            <div class="v">
              {{ data.days_left }} <span class="text-base font-semibold text-ink-soft">hari lagi</span>
            </div>
            <div class="d">
              {{ fmtLongDate(new Date(data.deadline)) }}
              <template v-if="data.pending_validation">
                · {{ data.pending_validation }} KPI belum divalidasi</template
              >
            </div>
          </template>
          <template v-else>
            <div class="v">—</div>
            <div class="d">
              periode {{ data.period.status === 'closed' ? 'sudah ditutup' : 'belum dibuka' }}
            </div>
          </template>
        </div>

        <div class="card">
          <div class="k">Status penilaian</div>
          <div class="v !text-xl">
            <span class="badge" :class="APPRAISAL_STATUS[data.appraisal_status]?.cls">{{
              APPRAISAL_STATUS[data.appraisal_status]?.label
            }}</span>
          </div>
          <div class="d"><RouterLink to="/penilaian/saya">Buka form penilaian →</RouterLink></div>
        </div>
      </div>

      <div class="flex flex-wrap items-start gap-6">
        <section class="card min-w-0 flex-[2_1_560px]">
          <div class="mb-4 flex flex-wrap items-start gap-3">
            <div class="flex-1">
              <h2>Progres KPI {{ data.period.name }}</h2>
              <p class="hint">Capaian terhadap target, berbobot</p>
            </div>
            <RouterLink to="/kpi" class="btn sm pri">Input realisasi</RouterLink>
          </div>
          <p v-if="!rows.length" class="muted m-0">Departemen Anda belum memiliki template KPI aktif.</p>
          <div
            v-for="r in rows"
            :key="r.indicator.id"
            class="border-b border-line-soft py-3 last:border-b-0"
            :data-testid="`progress-${r.indicator.id}`"
          >
            <div class="flex items-baseline justify-between gap-3">
              <div>
                <b>{{ r.indicator.name }}</b>
                <div class="muted text-xs">
                  <template v-if="r.result"
                    >{{ unitText(r.result.actual, r.indicator.unit) }} dari
                    {{ unitText(r.indicator.target, r.indicator.unit) }}</template
                  >
                  <template v-else
                    >belum diisi · target {{ unitText(r.indicator.target, r.indicator.unit) }}</template
                  >
                  · bobot {{ fmtNum(r.indicator.weight) }}%
                </div>
              </div>
              <b class="tabular-nums">{{ r.status === 'missing' ? '—' : fmtScore(r.achievement) }}</b>
            </div>
            <div
              class="mt-2 h-2 overflow-hidden rounded-full bg-line-soft"
              role="progressbar"
              :aria-valuenow="Number(r.achievement)"
              aria-valuemin="0"
              aria-valuemax="120"
              :aria-label="r.indicator.name"
            >
              <div
                class="h-full rounded-full"
                :class="Number(r.achievement) >= 100 ? 'bg-primary' : 'bg-accent'"
                :style="{ width: Math.min((Number(r.achievement) / 120) * 100, 100) + '%' }"
              />
            </div>
          </div>
          <p class="hint mb-0 mt-3">Bar penuh = capaian 120 (batas maksimum). Oranye = di bawah target.</p>
        </section>

        <section class="card min-w-0 flex-[1_1_320px]">
          <h2>Skor akhir 6 periode</h2>
          <p class="hint mb-3">70% KPI + 30% perilaku (hanya periode yang sudah final)</p>
          <BarChart
            :labels="history.map((h) => h.name.split(' ')[0] ?? h.name)"
            :values="history.map((h) => (h.score ? Number(h.score) : null))"
            :highlight="history.findIndex((h) => h.current)"
            description="Skor akhir enam periode terakhir"
          />
        </section>
      </div>
    </template>
  </PageState>
</template>
