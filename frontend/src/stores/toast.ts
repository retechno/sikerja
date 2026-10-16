import { ref } from 'vue'
import { defineStore } from 'pinia'

export type ToastKind = 'success' | 'error' | 'info'
export interface Toast {
  id: number
  kind: ToastKind
  text: string
}

let seq = 0

export const useToastStore = defineStore('toast', () => {
  const items = ref<Toast[]>([])

  function push(kind: ToastKind, text: string, ms = 4500) {
    const id = ++seq
    items.value.push({ id, kind, text })
    setTimeout(() => dismiss(id), ms)
  }

  function dismiss(id: number) {
    items.value = items.value.filter((t) => t.id !== id)
  }

  return {
    items,
    dismiss,
    success: (t: string) => push('success', t),
    error: (t: string) => push('error', t, 7000),
    info: (t: string) => push('info', t),
  }
})
