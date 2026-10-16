<script setup lang="ts">
import { computed } from 'vue'
import type { LeaveBalance } from '@/api'
import { fmtDays } from '@/lib/leave'

// Kartu saldo per jenis (H-08, dasbor). `extra` = hari pengajuan yang sedang diisi pada jenis `extraTypeId`.
const props = defineProps<{
  balances: LeaveBalance[]
  extra?: number
  extraTypeId?: number | null
  compact?: boolean
}>()

interface Row {
  b: LeaveBalance
  usedPct: number
  pendingPct: number
  extraPct: number
  after: number | null
}

const rows = computed<Row[]>(() =>
  props.balances.map((b) => {
    const quota = Number(b.quota ?? 0)
    const pct = (n: number) => (quota > 0 ? Math.min((n / quota) * 100, 100) : 0)
    const extra = props.extraTypeId === b.leave_type.id ? (props.extra ?? 0) : 0
    return {
      b,
      usedPct: pct(Number(b.used)),
      pendingPct: pct(Number(b.pending)),
      extraPct: pct(extra),
      after: b.tracked && extra > 0 ? Number(b.available ?? 0) - extra : null,
    }
  }),
)

// jenis tak berkuota tampil sebagai catatan singkat
const tracked = computed(() => rows.value.filter((r) => r.b.tracked))
const others = computed(() => rows.value.filter((r) => !r.b.tracked && Number(r.b.used) > 0))
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="balances">
    <div v-for="r in tracked" :key="r.b.leave_type.id" :data-testid="`bal-${r.b.leave_type.code}`">
      <div class="flex items-baseline justify-between gap-3">
        <b>{{ r.b.leave_type.name }}</b>
        <span
          ><b class="tabular-nums">{{ fmtDays(r.b.available) }}</b>
          <span class="muted"> / {{ fmtDays(r.b.quota) }} hari</span></span
        >
      </div>
      <div
        class="mt-1.5 flex h-2 overflow-hidden rounded-full bg-line-soft"
        role="img"
        :aria-label="`${r.b.leave_type.name}: ${fmtDays(r.b.used)} terpakai, ${fmtDays(r.b.pending)} menunggu`"
      >
        <i class="block h-full bg-primary" :style="{ width: r.usedPct + '%' }" />
        <i class="block h-full bg-accent" :style="{ width: r.pendingPct + '%' }" />
        <i v-if="r.extraPct" class="block h-full bg-[#B5D2CC]" :style="{ width: r.extraPct + '%' }" />
      </div>
      <div v-if="!compact" class="muted mt-1 text-xs">
        {{ fmtDays(r.b.used) }} terpakai · {{ fmtDays(r.b.pending) }} menunggu
        <template v-if="r.after !== null">
          · pengajuan ini {{ fmtDays(extra) }} → sisa
          <b :class="r.after < 0 ? 'text-bad-fg' : 'text-ink'">{{ fmtDays(r.after) }}</b></template
        >
      </div>
    </div>

    <div v-for="r in others" :key="r.b.leave_type.id" class="flex items-center justify-between gap-3">
      <b>{{ r.b.leave_type.name }}</b>
      <span class="muted text-xs">{{ fmtDays(r.b.used) }} hari tercatat · tidak memotong saldo</span>
    </div>
  </div>
</template>
