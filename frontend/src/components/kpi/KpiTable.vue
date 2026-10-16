<script setup lang="ts">
import { DIRECTION_LABEL, fmtNum, fmtScore, RESULT_STATUS } from '@/lib/format'
import type { KpiRow } from '@/api'

// Tabel KPI (H-05): target, realisasi, capaian, bobot, nilai, status. Dipakai juga di panel validasi manajer.
const props = defineProps<{
  rows: KpiRow[]
  score: string
  /** Pengguna boleh mengisi/mengubah realisasi (periode aktif, bukan final). */
  canInput: boolean
  selectedId?: number | null
  title?: string
  hint?: string
}>()
const emit = defineEmits<{ select: [row: KpiRow] }>()

function withUnit(v: unknown, unit: string, showUnit = true): string {
  const n = fmtNum(v)
  if (n === '—') return n
  if (unit === '%') return `${n}%`
  if (!showUnit || unit === 'dari 5' || !unit) return n
  return `${n} ${unit}`
}

const totalWeight = () => props.rows.reduce((a, r) => a + Number(r.indicator.weight), 0)
const editable = (r: KpiRow) => props.canInput && r.indicator.source === 'manual'
</script>

<template>
  <section class="card !p-2 pb-1">
    <div v-if="title" class="px-3.5 pb-3 pt-3">
      <h2>{{ title }}</h2>
      <p v-if="hint" class="hint">{{ hint }}</p>
    </div>
    <div class="overflow-x-auto">
      <table class="tbl" data-testid="kpi-table">
        <thead>
          <tr>
            <th>Indikator</th>
            <th class="num">Target</th>
            <th class="num">Realisasi</th>
            <th class="num">Capaian</th>
            <th class="num">Bobot</th>
            <th class="num">Nilai</th>
            <th>Status</th>
            <th><span class="sr-only">Aksi</span></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in rows"
            :key="r.indicator.id"
            :class="{ sel: selectedId === r.indicator.id }"
            :data-testid="`kpi-row-${r.indicator.id}`"
          >
            <td>
              <b>{{ r.indicator.name }}</b>
              <div class="muted text-xs">{{ DIRECTION_LABEL[r.indicator.direction] }}</div>
            </td>
            <td class="num">{{ withUnit(r.indicator.target, r.indicator.unit) }}</td>
            <td class="num" data-col="actual">
              {{ r.result ? withUnit(r.result.actual, r.indicator.unit, false) : '—' }}
            </td>
            <td class="num" data-col="achievement">
              {{ r.status === 'missing' ? '—' : fmtScore(r.achievement) }}
            </td>
            <td class="num">{{ fmtNum(r.indicator.weight) }}%</td>
            <td class="num" data-col="points">
              <b>{{ r.status === 'missing' ? '—' : fmtScore(r.points) }}</b>
            </td>
            <td>
              <span class="badge" :class="RESULT_STATUS[r.status]?.cls" data-col="status">{{
                RESULT_STATUS[r.status]?.label
              }}</span>
              <div
                v-if="r.status === 'rejected' && r.result?.validation_note"
                class="mt-1 max-w-[220px] text-xs text-bad-fg"
              >
                {{ r.result.validation_note }}
              </div>
            </td>
            <td class="text-right">
              <button
                type="button"
                class="btn sm"
                :class="{ pri: editable(r) && r.status !== 'validated' }"
                :data-testid="`kpi-action-${r.indicator.id}`"
                @click="emit('select', r)"
              >
                {{ editable(r) ? (r.status === 'missing' ? 'Isi' : 'Ubah') : 'Lihat' }}
              </button>
            </td>
          </tr>
        </tbody>
        <tfoot>
          <tr>
            <td colspan="4">Skor KPI</td>
            <td class="num" data-testid="kpi-total-weight">{{ fmtNum(totalWeight()) }}%</td>
            <td class="num" data-testid="kpi-total-score">{{ fmtScore(score) }}</td>
            <td colspan="2"></td>
          </tr>
        </tfoot>
      </table>
    </div>
  </section>
</template>
