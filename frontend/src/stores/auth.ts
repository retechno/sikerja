import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { auth as authApi, type Employee, type Role } from '@/api'
import { onSessionExpired } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<Employee | null>(null)
  const loaded = ref(false)

  const roles = computed<Role[]>(() => user.value?.roles ?? [])
  const has = (...r: Role[]) => r.some((x) => roles.value.includes(x))
  const isEmployee = computed(() => has('employee'))
  const isManager = computed(() => has('manager'))
  const isHR = computed(() => has('hr'))
  const isDirector = computed(() => has('director'))

  /** Halaman awal menurut role utama. */
  const home = computed(() => {
    if (isDirector.value) return '/dashboard/direktur'
    if (isHR.value) return '/master/karyawan'
    if (isManager.value) return '/dashboard/tim'
    return '/dashboard'
  })

  /** Muat profil dari cookie sesi; false bila belum login. */
  async function fetchMe(): Promise<boolean> {
    try {
      user.value = await authApi.me()
      return true
    } catch {
      user.value = null
      return false
    } finally {
      loaded.value = true
    }
  }

  async function login(email: string, password: string, remember: boolean) {
    user.value = await authApi.login(email, password, remember)
    loaded.value = true
  }

  async function logout() {
    try {
      await authApi.logout()
    } finally {
      user.value = null
    }
  }

  function setUser(u: Employee) {
    user.value = u
  }

  function clear() {
    user.value = null
  }

  return { user, loaded, roles, has, isEmployee, isManager, isHR, isDirector, home, fetchMe, login, logout, setUser, clear }
})

/** Hubungkan klien HTTP: bila refresh gagal, bersihkan sesi dan kembali ke halaman login. */
export function bindSessionExpiry(redirect: () => void) {
  onSessionExpired(() => {
    useAuthStore().clear()
    redirect()
  })
}
