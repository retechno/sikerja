import { ref } from 'vue'
import { defineStore } from 'pinia'
import { leave } from '@/api'

/** Jumlah pengajuan yang menunggu keputusan pengguna (lencana menu "Persetujuan"). */
export const useLeaveStore = defineStore('leave', () => {
  const pending = ref(0)

  async function refresh() {
    try {
      pending.value = (await leave.approvalCount()).pending
    } catch {
      /* lencana tidak boleh mengganggu halaman (mis. karyawan biasa tidak punya antrean) */
    }
  }

  function reset() {
    pending.value = 0
  }

  return { pending, refresh, reset }
})
