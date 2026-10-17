import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { periods as periodsApi, type Period } from '@/api'

const KEY = 'sikerja.period'

function readSaved(): number | null {
  try {
    const v = Number(localStorage.getItem(KEY))
    return Number.isFinite(v) && v > 0 ? v : null
  } catch {
    return null
  }
}

function save(id: number | null) {
  try {
    if (id) localStorage.setItem(KEY, String(id))
  } catch {
    /* penyimpanan bisa diblokir; tidak masalah */
  }
}

export const usePeriodStore = defineStore('period', () => {
  const list = ref<Period[]>([])
  const selectedId = ref<number | null>(null)
  const loading = ref(false)
  let inflight: Promise<void> | null = null

  const selected = computed(() => list.value.find((p) => p.id === selectedId.value) ?? null)
  /** Periode aktif terbaru (untuk default & tombol input realisasi). */
  const active = computed(() => list.value.find((p) => p.status === 'active') ?? null)

  async function fetchAll() {
    loading.value = true
    try {
      const all = await periodsApi.list()
      list.value = [...all].sort((a, b) => b.start_date.localeCompare(a.start_date))
      const saved = readSaved()
      const keep = list.value.find((p) => p.id === (selectedId.value ?? saved))
      const pick =
        keep ??
        list.value.find((p) => p.status === 'active') ??
        list.value.find((p) => p.status !== 'draft') ??
        list.value[0]
      selectedId.value = pick?.id ?? null
      save(selectedId.value)
    } finally {
      loading.value = false
    }
  }

  /** Memuat daftar periode sekali; panggilan bersamaan berbagi satu promise sehingga bisa di-await. */
  function load(force = false): Promise<void> {
    if (inflight) return inflight
    if (list.value.length && !force) return Promise.resolve()
    inflight = fetchAll().finally(() => {
      inflight = null
    })
    return inflight
  }

  function select(id: number) {
    selectedId.value = id
    save(id)
  }

  function reset() {
    list.value = []
    selectedId.value = null
  }

  return { list, selectedId, selected, active, loading, load, select, reset }
})
