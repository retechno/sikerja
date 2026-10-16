<script setup lang="ts">
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import type { ChartOptions } from 'chart.js'
import { COLORS } from './chart'
import { fmtScore } from '@/lib/format'

const props = defineProps<{
  labels: string[]
  values: (number | null)[]
  min?: number
  max?: number
  height?: number
  ariaLabel: string
}>()

const data = computed(() => ({
  labels: props.labels,
  datasets: [
    {
      data: props.values,
      borderColor: COLORS.primary,
      backgroundColor: 'rgba(14,90,82,0.10)',
      pointBackgroundColor: COLORS.primary,
      fill: true,
      tension: 0.3,
      spanGaps: true,
    },
  ],
}))

const options = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false }, tooltip: { callbacks: { label: (c) => ` ${fmtScore(c.parsed.y)}` } } },
  scales: {
    x: { grid: { display: false } },
    y: { min: props.min ?? 60, max: props.max ?? 100, grid: { color: COLORS.line } },
  },
}))
</script>

<template>
  <div :style="{ height: (height ?? 220) + 'px' }" role="img" :aria-label="ariaLabel">
    <Line :data="data" :options="options" />
  </div>
</template>
