import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import type { Role } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    /** Role yang boleh membuka (salah satu). Kosong = semua pengguna yang login. */
    roles?: Role[]
    public?: boolean
    /** Tampilkan pemilih periode di header. */
    period?: boolean
  }
}

export const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/pages/auth/LoginPage.vue'), meta: { public: true, title: 'Masuk' } },
  {
    path: '/forgot-password',
    name: 'forgot',
    component: () => import('@/pages/auth/ForgotPasswordPage.vue'),
    meta: { public: true, title: 'Lupa password' },
  },
  {
    path: '/reset-password',
    name: 'reset',
    component: () => import('@/pages/auth/ResetPasswordPage.vue'),
    meta: { public: true, title: 'Atur ulang password' },
  },
  {
    path: '/',
    component: () => import('@/components/layout/AppLayout.vue'),
    children: [
      { path: '', name: 'home', redirect: () => useAuthStore().home },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/pages/dashboard/EmployeeDashboard.vue'),
        meta: { title: 'Dashboard', roles: ['employee'], period: true },
      },
      {
        path: 'dashboard/tim',
        name: 'dashboard-tim',
        component: () => import('@/pages/dashboard/ManagerDashboard.vue'),
        meta: { title: 'Dashboard Tim', roles: ['manager', 'director'], period: true },
      },
      {
        path: 'dashboard/direktur',
        name: 'dashboard-direktur',
        component: () => import('@/pages/dashboard/DirectorDashboard.vue'),
        meta: { title: 'Kinerja Perusahaan', roles: ['director', 'hr'], period: true },
      },
      {
        path: 'kpi',
        name: 'kpi',
        component: () => import('@/pages/kpi/MyKpiPage.vue'),
        meta: { title: 'KPI Saya', roles: ['employee'], period: true },
      },
      {
        path: 'penilaian/saya',
        name: 'penilaian-saya',
        component: () => import('@/pages/appraisal/AppraisalPage.vue'),
        props: { self: true },
        meta: { title: 'Penilaian Saya', roles: ['employee'], period: true },
      },
      {
        path: 'penilaian/tim',
        name: 'penilaian-tim',
        component: () => import('@/pages/appraisal/TeamAppraisalPage.vue'),
        meta: { title: 'Penilaian Tim', roles: ['manager', 'director'], period: true },
      },
      {
        path: 'penilaian/:employeeId(\\d+)',
        name: 'penilaian-detail',
        component: () => import('@/pages/appraisal/AppraisalPage.vue'),
        meta: { title: 'Form Penilaian', roles: ['manager', 'director', 'hr'], period: true },
      },
      {
        path: 'master/karyawan',
        name: 'karyawan',
        component: () => import('@/pages/master/EmployeesPage.vue'),
        meta: { title: 'Karyawan', roles: ['hr'] },
      },
      {
        path: 'master/template-kpi',
        name: 'template-kpi',
        component: () => import('@/pages/master/KpiTemplatesPage.vue'),
        meta: { title: 'Template KPI', roles: ['hr', 'director'] },
      },
      {
        path: 'master/periode',
        name: 'periode',
        component: () => import('@/pages/master/PeriodsPage.vue'),
        meta: { title: 'Periode Penilaian', roles: ['hr'] },
      },
      {
        path: 'laporan',
        name: 'laporan',
        component: () => import('@/pages/report/ReportPage.vue'),
        meta: { title: 'Laporan', roles: ['hr', 'director'], period: true },
      },
      {
        path: 'pengaturan',
        name: 'pengaturan',
        component: () => import('@/pages/settings/SettingsPage.vue'),
        meta: { title: 'Pengaturan', roles: ['hr'] },
      },
      { path: 'profil', name: 'profil', component: () => import('@/pages/profile/ProfilePage.vue'), meta: { title: 'Profil' } },
      { path: ':pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/NotFoundPage.vue'), meta: { title: 'Tidak ditemukan' } },
    ],
  },
]

export function createAppRouter() {
  const router = createRouter({ history: createWebHistory(), routes, scrollBehavior: () => ({ top: 0 }) })

  router.beforeEach(async (to) => {
    const auth = useAuthStore()
    if (to.meta.public) {
      // pengguna yang sudah login tidak perlu melihat halaman login lagi
      if (to.name === 'login') {
        if (!auth.loaded) await auth.fetchMe()
        if (auth.user) return auth.home
      }
      return true
    }
    if (!auth.loaded) await auth.fetchMe()
    if (!auth.user) return { name: 'login', query: to.fullPath !== '/' ? { redirect: to.fullPath } : {} }

    const need = to.meta.roles
    if (need && !auth.has(...need)) {
      useToastStore().error('Anda tidak berhak membuka halaman itu.')
      return auth.home
    }
    return true
  })

  router.afterEach((to) => {
    document.title = to.meta.title ? `${to.meta.title} · SiKerja` : 'SiKerja'
  })
  return router
}
