<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import Avatar from '@/components/ui/Avatar.vue'
import PendingApprovals from '@/components/leave/PendingApprovals.vue'
import { dashboard, leave } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtNum, fmtScore, gradeClass } from '@/lib/format'
import { usePeriodStore } from '@/stores/period'
import { useAuthStore } from '@/stores/auth'

// H-03 — Dashboard Manajer
const period = usePeriodStore()
const auth = useAuthStore()
const { data, loading, error, reload } = useLoader(async () => {
  await period.load()
  return dashboard.manager(period.selectedId ?? undefined)
}, [() => period.selectedId])

// pengajuan izin/cuti yang menunggu persetujuan (tidak di-cache; dimuat terpisah)
const leaveSummary = useLoader(() => leave.summary())

const delta = computed(() => (data.value?.prev_avg_delta != null ? Number(data.value.prev_avg_delta) : null))
const heading = computed(() => (auth.isManager ? `Tim ${auth.user?.department?.name ?? ''}` : 'Manajer saya'))
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="data">
      <div>
        <h2 class="m-0 text-xl font-bold">{{ heading }}</h2>
        <p class="hint">Ringkasan kinerja dan tugas yang menunggu Anda · {{ data.period.name }}</p>
      </div>

      <div class="grid grid-cols-[repeat(auto-fit,minmax(230px,1fr))] gap-4">
        <div class="card" data-testid="avg-card">
          <div class="k">Rata-rata skor tim · {{ data.prev_period_name || '—' }}</div>
          <div class="v">{{ fmtScore(data.prev_avg) }}</div>
          <div class="d">
            <template v-if="delta !== null"
              ><span :class="delta >= 0 ? 'up' : 'down'"
                >{{ delta >= 0 ? '+' : '−' }}{{ fmtScore(Math.abs(delta)) }}</span
              >
              dari periode sebelumnya</template
            >
            <template v-else>skor final periode terakhir</template>
          </div>
        </div>
        <div class="card" data-testid="pending-card">
          <div class="k">Realisasi menunggu validasi</div>
          <div class="v">{{ data.pending_validations }}</div>
          <div class="d">
            dari {{ data.members_pending }} anggota · <RouterLink to="/penilaian/tim">validasi</RouterLink>
          </div>
        </div>
        <div class="card" data-testid="leave-card">
          <div class="k">Persetujuan izin &amp; cuti</div>
          <div class="v">{{ leaveSummary.data.value?.pending_count ?? '—' }}</div>
          <div class="d">menunggu keputusan · <RouterLink to="/persetujuan">buka antrean</RouterLink></div>
        </div>
        <div class="card">
          <div class="k">Belum dinilai atasan</div>
          <div class="v">{{ data.not_appraised }}</div>
          <div class="d">
            dari {{ data.members_total }} anggota · <RouterLink to="/penilaian/tim">nilai</RouterLink>
          </div>
        </div>
        <div class="card" data-testid="low-card">
          <div class="k">Anggota skor &lt; 70</div>
          <div class="v">{{ data.low_members.length }}</div>
          <div class="d">
            <template v-if="data.low_members[0]"
              >{{ data.low_members[0].employee.name }} · {{ fmtScore(data.low_members[0].score) }}</template
            >
            <template v-else>tidak ada</template>
          </div>
        </div>
      </div>

      <PendingApprovals
        v-if="leaveSummary.data.value && leaveSummary.data.value.pending_count > 0"
        :items="leaveSummary.data.value.queue"
        :total="leaveSummary.data.value.pending_count"
      />

      <div class="flex flex-wrap items-start gap-6">
        <section class="card min-w-0 flex-[2_1_560px] !p-2 pb-1">
          <div class="px-3.5 pb-3 pt-3">
            <h2>Peringkat tim</h2>
            <p class="hint">
              Skor akhir {{ data.prev_period_name || 'periode lalu' }} (final) dan skor KPI
              {{ data.period.name }} (sementara)
            </p>
          </div>
          <div class="overflow-x-auto">
            <table class="tbl">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Anggota</th>
                  <th class="num">{{ data.prev_period_name?.split(' ')[0] ?? 'Lalu' }}</th>
                  <th>Predikat</th>
                  <th class="num">KPI sementara</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(r, i) in data.ranking" :key="r.employee.id">
                  <td>{{ i + 1 }}</td>
                  <td>
                    <RouterLink
                      :to="`/penilaian/${r.employee.id}`"
                      class="flex items-center gap-2.5 text-ink no-underline hover:text-primary"
                    >
                      <Avatar :employee="r.employee" size="s" /> <b>{{ r.employee.name }}</b>
                    </RouterLink>
                  </td>
                  <td class="num">{{ fmtScore(r.prev_score) }}</td>
                  <td>
                    <span v-if="r.prev_grade" class="badge" :class="gradeClass(r.prev_grade)">{{
                      r.prev_grade
                    }}</span>
                  </td>
                  <td class="num">{{ fmtScore(r.kpi_score) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="card min-w-0 flex-[1_1_340px]" data-testid="below-target">
          <h2>KPI di bawah target</h2>
          <p class="hint mb-3">{{ data.period.name }}, capaian &lt; 100</p>
          <p v-if="!data.below_target.length" class="muted m-0">
            Semua KPI yang sudah diisi mencapai target.
          </p>
          <div
            v-for="(b, i) in data.below_target"
            :key="i"
            class="flex items-start gap-3 border-b border-line-soft py-2.5 last:border-b-0"
          >
            <Avatar :employee="b.employee" size="s" />
            <div class="min-w-0 flex-1">
              <b>{{ b.indicator }}</b>
              <div class="muted text-xs">
                {{ b.employee.name }} · {{ fmtNum(b.actual) }}{{ b.unit === '%' ? '%' : '' }} dari target
                {{ b.direction === 'lower' ? '≤ ' : '' }}{{ fmtNum(b.target) }}{{ b.unit === '%' ? '%' : '' }}
              </div>
            </div>
            <span class="badge b-warn">{{ fmtScore(b.achievement) }}</span>
          </div>
        </section>
      </div>
    </template>
  </PageState>
</template>
