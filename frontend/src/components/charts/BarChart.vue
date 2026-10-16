<script setup lang="ts">
import { computed } from 'vue'
import { Bar } from 'vue-chartjs'
import type { ChartOptions } from 'chart.js'
import { COLORS } from './chart'
import { fmtScore } from '@/lib/format'

// Batang tunggal-seri. Nilai null (belum ada) digambar kosong. `highlight` = indeks batang yang ditonjolkan.
const props = defineProps<{
  labels: string[]
  values: (number | null)[]
  highlight?: number
  horizontal?: boolean
  max?: number
  colors?: string[]
  height?: number
  description: string
}>()

const data = computed(() => ({
  labels: props.labels,
  datasets: [
    {
      data: props.values,
      backgroundColor: props.values.map(
        (_, i) => props.colors?.[i] ?? (i === props.highlight ? COLORS.primary : COLORS.soft),
      ),
      borderRadius: 6,
      maxBarThickness: 44,
    },
  ],
}))

const options = computed<ChartOptions<'bar'>>(() => {
  const valueAxis = { min: 0, max: props.max ?? 100, grid: { color: COLORS.line } }
  const catAxis = { grid: { display: false } }
  return {
    indexAxis: props.horizontal ? 'y' : 'x',
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      tooltip: { callbacks: { label: (c) => ` ${fmtScore(c.parsed[props.horizontal ? 'x' : 'y'])}` } },
    },
    scales: props.horizontal ? { x: valueAxis, y: catAxis } : { x: catAxis, y: valueAxis },
  }
})
</script>

<template>
  <div :style="{ height: (height ?? 220) + 'px' }" role="img" :aria-label="description">
    <Bar :data="data" :options="options" />
  </div>
</template>
