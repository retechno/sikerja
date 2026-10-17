<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import Icon from '@/components/ui/Icon.vue'
import Avatar from '@/components/ui/Avatar.vue'
import NotificationBell from '@/components/layout/NotificationBell.vue'
import { useAuthStore } from '@/stores/auth'
import { usePeriodStore } from '@/stores/period'
import { useLeaveStore } from '@/stores/leave'
import { ROLE_LABEL } from '@/lib/format'

interface NavItem {
  to: string
  label: string
  icon: string
  /** tampilkan lencana jumlah persetujuan menunggu */
  badge?: boolean
}
interface NavGroup {
  label: string
  items: NavItem[]
}

const auth = useAuthStore()
const periods = usePeriodStore()
const leaveStore = useLeaveStore()
const route = useRoute()
const router = useRouter()
const drawer = ref(false)
const menu = ref(false)

// Menu sesuai role. Izin & cuti tersedia untuk semua; Persetujuan untuk manajer, HR, dan Direktur.
const leaveItems: NavItem[] = [
  { to: '/izin-cuti/ajukan', label: 'Ajukan Izin/Cuti', icon: 'send' },
  { to: '/izin-cuti/riwayat', label: 'Riwayat Pengajuan', icon: 'clock' },
  { to: '/kalender', label: 'Kalender Tim', icon: 'calendar' },
]
const approvalItem: NavItem = { to: '/persetujuan', label: 'Persetujuan', icon: 'inbox', badge: true }
const groups = computed<NavGroup[]>(() => {
  const g: NavGroup[] = []
  if (auth.isEmployee) {
    g.push({
      label: auth.isManager ? 'Saya' : 'Menu',
      items: [
        { to: '/dashboard', label: 'Dashboard', icon: 'home' },
        { to: '/kpi', label: 'KPI Saya', icon: 'target' },
        { to: '/penilaian/saya', label: 'Penilaian Saya', icon: 'clipboard' },
        ...leaveItems,
      ],
    })
  } else {
    g.push({ label: 'Izin & Cuti', items: leaveItems })
  }
  if (auth.isManager || (auth.isDirector && !auth.isManager)) {
    g.push({
      label: auth.isDirector && !auth.isManager ? 'Manajer saya' : 'Tim saya',
      items: [
        ...(auth.isManager ? [{ to: '/dashboard/tim', label: 'Dashboard Tim', icon: 'home' }] : []),
        {
          to: '/penilaian/tim',
          label: auth.isManager ? 'Penilaian Tim' : 'Penilaian Manajer',
          icon: 'clipboard',
        },
        approvalItem,
      ],
    })
  }
  if (auth.isDirector) {
    g.unshift({
      label: 'Perusahaan',
      items: [
        { to: '/dashboard/direktur', label: 'Dashboard', icon: 'home' },
        { to: '/laporan', label: 'Laporan', icon: 'chart' },
        { to: '/master/template-kpi', label: 'Template KPI', icon: 'list' },
      ],
    })
  }
  if (auth.isHR) {
    g.push({
      label: 'Administrasi',
      items: [
        approvalItem,
        { to: '/master/karyawan', label: 'Karyawan', icon: 'users' },
        { to: '/master/template-kpi', label: 'Template KPI', icon: 'list' },
        { to: '/master/periode', label: 'Periode Penilaian', icon: 'clock' },
        { to: '/laporan', label: 'Laporan', icon: 'chart' },
        { to: '/pengaturan', label: 'Pengaturan', icon: 'gear' },
      ],
    })
  }
  // hapus duplikat tautan antar-grup (mis. Laporan untuk Direktur sekaligus HR)
  const seen = new Set<string>()
  return g
    .map((grp) => ({ ...grp, items: grp.items.filter((i) => !seen.has(i.to) && seen.add(i.to)) }))
    .filter((grp) => grp.items.length)
})

const roleLine = computed(() => {
  const u = auth.user
  if (!u) return ''
  const main = auth.isDirector ? 'director' : auth.isHR ? 'hr' : auth.isManager ? 'manager' : 'employee'
  const dept = u.department?.name
  return `${ROLE_LABEL[main]}${dept ? ' · ' + dept : ''}`
})

const showPeriod = computed(() => route.meta.period && periods.list.length > 0)

const approver = computed(() => auth.isManager || auth.isHR || auth.isDirector)

// tab bawah di ponsel (M08): jalan pintas ke fitur yang paling sering dipakai
const tabs = computed<NavItem[]>(() => [
  { to: auth.home, label: 'Beranda', icon: 'home' },
  auth.isEmployee
    ? { to: '/kpi', label: 'KPI', icon: 'target' }
    : { to: '/persetujuan', label: 'Setuju', icon: 'inbox', badge: true },
  { to: '/izin-cuti/ajukan', label: 'Cuti', icon: 'send' },
  { to: '/kalender', label: 'Kalender', icon: 'calendar' },
  { to: '/profil', label: 'Profil', icon: 'user' },
])

onMounted(() => {
  void periods.load()
  if (approver.value) void leaveStore.refresh()
})
watch(
  () => route.fullPath,
  () => {
    drawer.value = false
    menu.value = false
    if (approver.value) void leaveStore.refresh()
  },
)

async function logout() {
  await auth.logout()
  periods.reset()
  leaveStore.reset()
  await router.replace({ name: 'login' })
}

function onSelectPeriod(e: Event) {
  periods.select(Number((e.target as HTMLSelectElement).value))
}

const periodLabel = (p: { name: string; status: string }) =>
  `${p.name}${p.status === 'active' ? ' (aktif)' : p.status === 'draft' ? ' (draf)' : ''}`
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Sidebar (desktop) / drawer (ponsel) -->
    <div
      v-if="drawer"
      class="fixed inset-0 z-30 bg-black/40 lg:hidden"
      aria-hidden="true"
      @click="drawer = false"
    />
    <aside
      class="fixed inset-y-0 left-0 z-40 flex w-[264px] flex-col gap-6 overflow-y-auto bg-side p-4 pt-6 text-side-text transition-transform lg:sticky lg:top-0 lg:h-screen lg:translate-x-0"
      :class="drawer ? 'translate-x-0' : '-translate-x-full'"
      aria-label="Menu utama"
    >
      <div class="flex items-center justify-between px-2">
        <RouterLink
          to="/"
          class="flex items-center gap-2.5 text-[19px] font-extrabold text-white no-underline hover:text-white"
        >
          <span
            class="flex h-[34px] w-[34px] items-center justify-center rounded-[10px] bg-accent font-extrabold text-side"
            >S</span
          >
          SiKerja
        </RouterLink>
        <button
          class="rounded-lg p-2 text-side-text lg:hidden"
          type="button"
          aria-label="Tutup menu"
          @click="drawer = false"
        >
          <Icon name="x" :size="20" />
        </button>
      </div>

      <nav v-for="grp in groups" :key="grp.label">
        <p class="mb-1.5 px-3 text-[11px] uppercase tracking-[0.08em] text-side-label">{{ grp.label }}</p>
        <div class="flex flex-col gap-0.5">
          <RouterLink
            v-for="it in grp.items"
            :key="it.to"
            :to="it.to"
            class="flex min-h-[44px] items-center gap-3 rounded-[10px] px-3 py-2.5 font-medium text-side-text no-underline hover:bg-side-hover hover:text-white"
            active-class="!bg-primary-soft !font-bold !text-side"
          >
            <Icon :name="it.icon" />{{ it.label }}
            <span
              v-if="it.badge && leaveStore.pending > 0"
              class="ml-auto rounded-full bg-accent px-2 py-px text-[11px] font-bold text-side"
              :aria-label="`${leaveStore.pending} menunggu`"
              data-testid="nav-badge"
              >{{ leaveStore.pending }}</span
            >
          </RouterLink>
        </div>
      </nav>

      <div class="mt-auto rounded-xl bg-side-hover px-3.5 py-3 text-xs text-[#B7C7C2]">
        <b class="block text-[13px] text-white">{{ auth.user?.name }}</b>
        {{ roleLine }}
      </div>
    </aside>

    <main class="flex min-w-0 flex-1 flex-col">
      <header
        class="flex flex-wrap items-center gap-3 border-b border-line bg-white px-4 py-3 sm:px-8 sm:py-[18px]"
      >
        <button
          class="grid h-11 w-11 place-items-center rounded-xl border border-line bg-white lg:hidden"
          type="button"
          aria-label="Buka menu"
          @click="drawer = true"
        >
          <Icon name="menu" :size="20" />
        </button>
        <h1 class="m-0 min-w-0 flex-1 truncate text-lg font-bold tracking-tight sm:text-[22px]">
          {{ route.meta.title }}
        </h1>

        <label v-if="showPeriod" class="flex items-center gap-2 text-[13px] font-semibold">
          <span class="hidden sm:inline">Periode</span>
          <select
            class="input !min-h-[44px] !w-auto"
            :value="periods.selectedId ?? ''"
            aria-label="Pilih periode"
            @change="onSelectPeriod"
          >
            <option
              v-for="p in periods.list.filter((x) => x.status !== 'draft' || auth.isHR)"
              :key="p.id"
              :value="p.id"
            >
              {{ periodLabel(p) }}
            </option>
          </select>
        </label>

        <NotificationBell />

        <div class="relative">
          <button
            class="flex min-h-[44px] items-center gap-2.5 rounded-xl px-1 text-left"
            type="button"
            :aria-expanded="menu"
            @click="menu = !menu"
          >
            <Avatar :employee="auth.user" />
            <span class="hidden leading-tight sm:block">
              <b class="block text-sm">{{ auth.user?.name }}</b>
              <span class="text-xs text-ink-soft">{{ auth.user?.position?.name ?? roleLine }}</span>
            </span>
          </button>
          <div
            v-if="menu"
            class="absolute right-0 top-[52px] z-20 w-52 overflow-hidden rounded-xl border border-line bg-white py-1 shadow-lg"
          >
            <RouterLink
              to="/profil"
              class="flex min-h-[44px] items-center gap-2.5 px-4 text-ink no-underline hover:bg-canvas hover:text-ink"
            >
              <Icon name="user" />Profil &amp; password
            </RouterLink>
            <button
              type="button"
              class="flex min-h-[44px] w-full items-center gap-2.5 px-4 text-left hover:bg-canvas"
              @click="logout"
            >
              <Icon name="logout" />Keluar
            </button>
          </div>
        </div>
      </header>

      <div class="mx-auto flex w-full max-w-[1240px] flex-col gap-6 px-4 pb-28 pt-6 sm:px-8 sm:pt-7 lg:pb-10">
        <RouterView />
      </div>

      <nav
        class="fixed inset-x-0 bottom-0 z-20 flex border-t border-line bg-white px-1 pb-3 pt-1 lg:hidden"
        aria-label="Navigasi cepat"
        data-testid="tabbar"
      >
        <RouterLink
          v-for="t in tabs"
          :key="t.to + t.label"
          :to="t.to"
          class="relative flex min-h-[52px] flex-1 flex-col items-center justify-center gap-0.5 text-[11px] text-ink-soft no-underline hover:text-ink-soft"
          active-class="!font-bold !text-primary"
        >
          <Icon :name="t.icon" :size="22" />{{ t.label }}
          <span
            v-if="t.badge && leaveStore.pending > 0"
            class="absolute right-[calc(50%-18px)] top-1 rounded-full bg-accent px-1.5 text-[10px] font-bold text-side"
            >{{ leaveStore.pending }}</span
          >
        </RouterLink>
      </nav>
    </main>
  </div>
</template>
