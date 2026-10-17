import { onMounted, ref, watch, type Ref, type WatchSource } from 'vue'
import { ApiError } from '@/api'

/**
 * Memuat data async dengan status loading/error. `sources` memicu muat ulang (mis. periode berubah).
 * Respons yang tertinggal (kalah balapan) diabaikan.
 */
export function useLoader<T>(fn: () => Promise<T>, sources: WatchSource[] = []) {
  const data = ref<T | null>(null) as Ref<T | null>
  const loading = ref(true)
  const error = ref<string | null>(null)
  let seq = 0

  async function load() {
    const my = ++seq
    loading.value = true
    error.value = null
    try {
      const r = await fn()
      if (my === seq) data.value = r
    } catch (e) {
      if (my === seq) error.value = e instanceof ApiError ? e.message : 'Gagal memuat data'
    } finally {
      if (my === seq) loading.value = false
    }
  }

  onMounted(load)
  if (sources.length) watch(sources, load)
  return { data, loading, error, reload: load }
}
