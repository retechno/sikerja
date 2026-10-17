<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import Avatar from '@/components/ui/Avatar.vue'
import Modal from '@/components/ui/Modal.vue'
import Icon from '@/components/ui/Icon.vue'
import EmployeeFormModal from './EmployeeFormModal.vue'
import ImportModal from './ImportModal.vue'
import { ApiError, employees, master, type Department, type Employee, type PageMeta } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { ROLE_LABEL } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'

// H-12 — Karyawan (HR): daftar, tambah/ubah, import Excel, atur atasan & role.
const auth = useAuthStore()
const toast = useToastStore()

const q = ref('')
const deptFilter = ref<number | ''>('')
const statusFilter = ref('')
const page = ref(1)
const perPage = 10
const debounced = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(q, (v) => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    debounced.value = v
    page.value = 1
  }, 300)
})
watch([deptFilter, statusFilter], () => (page.value = 1))
onBeforeUnmount(() => clearTimeout(timer))

const { data, loading, error, reload } = useLoader(async () => {
  const [list, depts, everyone] = await Promise.all([
    employees.list({
      q: debounced.value || undefined,
      department_id: deptFilter.value || undefined,
      status: statusFilter.value || undefined,
      page: page.value,
      per_page: perPage,
    }),
    master.departments(),
    employees.list({ per_page: 200 }), // untuk pilihan atasan & nama atasan
  ])
  return { rows: list.data, meta: list.meta as PageMeta, depts: depts as Department[], people: everyone.data }
}, [debounced, deptFilter, statusFilter, page])

const nameOf = computed(() => {
  const m = new Map<number, string>()
  for (const p of data.value?.people ?? []) m.set(p.id, p.name)
  return m
})
const totalPages = computed(() => Math.max(1, Math.ceil((data.value?.meta.total ?? 0) / perPage)))
const from = computed(() => (data.value && data.value.meta.total ? (page.value - 1) * perPage + 1 : 0))
const to = computed(() => Math.min(page.value * perPage, data.value?.meta.total ?? 0))

// --- form ---
const formOpen = ref(false)
const editing = ref<Employee | null>(null)
function openForm(e: Employee | null) {
  editing.value = e
  formOpen.value = true
}

const created = ref<{ name: string; password: string } | null>(null)
async function onSaved(temp: string, name: string) {
  formOpen.value = false
  toast.success(editing.value ? 'Data karyawan diperbarui.' : 'Karyawan ditambahkan.')
  if (temp) created.value = { name, password: temp }
  await reload()
}

// --- hapus ---
const removing = ref<Employee | null>(null)
const busy = ref(false)
async function confirmRemove() {
  if (!removing.value) return
  busy.value = true
  try {
    await employees.remove(removing.value.id)
    toast.success(`${removing.value.name} dinonaktifkan dan dihapus dari daftar.`)
    removing.value = null
    await reload()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menghapus')
    removing.value = null
  } finally {
    busy.value = false
  }
}

const importOpen = ref(false)
</script>

<template>
  <div class="flex flex-wrap items-end gap-3">
    <label class="field min-w-[220px] flex-1">
      <span class="sr-only">Cari karyawan</span>
      <span class="relative block">
        <Icon
          name="search"
          class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-soft"
        />
        <input
          v-model="q"
          type="search"
          class="input !pl-10"
          placeholder="Cari nama atau NIK"
          data-testid="search"
        />
      </span>
    </label>
    <label class="field">
      <span class="sr-only">Departemen</span>
      <select v-model="deptFilter" class="input" data-testid="dept-filter">
        <option value="">Semua departemen</option>
        <option v-for="d in data?.depts ?? []" :key="d.id" :value="d.id">{{ d.name }}</option>
      </select>
    </label>
    <label class="field">
      <span class="sr-only">Status</span>
      <select v-model="statusFilter" class="input">
        <option value="">Semua status</option>
        <option value="active">Aktif</option>
        <option value="inactive">Nonaktif</option>
      </select>
    </label>
    <button type="button" class="btn" data-testid="open-import" @click="importOpen = true">
      <Icon name="upload" />Import Excel
    </button>
    <button type="button" class="btn pri" data-testid="open-add" @click="openForm(null)">
      <Icon name="plus" />Tambah karyawan
    </button>
  </div>

  <PageState
    :loading="loading && !data"
    :error="error"
    :empty="!!data && data.rows.length === 0"
    empty-text="Tidak ada karyawan yang cocok."
    @retry="reload"
  >
    <section v-if="data" class="card !p-2 pb-1" :class="{ 'opacity-60': loading }">
      <div class="overflow-x-auto">
        <table class="tbl" data-testid="employee-table">
          <thead>
            <tr>
              <th>NIK</th>
              <th>Nama</th>
              <th>Departemen</th>
              <th>Jabatan</th>
              <th>Atasan</th>
              <th>Role</th>
              <th>Status</th>
              <th><span class="sr-only">Aksi</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in data.rows" :key="e.id">
              <td class="whitespace-nowrap">{{ e.nik }}</td>
              <td>
                <div class="flex items-center gap-2.5">
                  <Avatar :employee="e" size="s" />
                  <div>
                    <b>{{ e.name }}</b>
                    <div class="muted text-xs">{{ e.email }}</div>
                  </div>
                </div>
              </td>
              <td>{{ e.department?.name ?? '—' }}</td>
              <td>{{ e.position?.name ?? '—' }}</td>
              <td>{{ e.manager_id ? (nameOf.get(e.manager_id) ?? '—') : '—' }}</td>
              <td>
                <span
                  v-for="r in e.roles"
                  :key="r"
                  class="badge mr-1"
                  :class="r === 'employee' ? 'b-mute' : 'b-teal'"
                  >{{ ROLE_LABEL[r] }}</span
                >
              </td>
              <td>
                <span class="badge" :class="e.status === 'active' ? 'b-good' : 'b-bad'">{{
                  e.status === 'active' ? 'Aktif' : 'Nonaktif'
                }}</span>
              </td>
              <td class="whitespace-nowrap text-right">
                <button
                  type="button"
                  class="btn sm mr-1.5"
                  :aria-label="`Ubah ${e.name}`"
                  @click="openForm(e)"
                >
                  <Icon name="edit" :size="15" />Ubah
                </button>
                <button
                  v-if="e.id !== auth.user?.id"
                  type="button"
                  class="btn sm"
                  :aria-label="`Hapus ${e.name}`"
                  @click="removing = e"
                >
                  <Icon name="trash" :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3 px-3.5 py-3 text-[13px] text-ink-soft">
        <span data-testid="range">Menampilkan {{ from }}–{{ to }} dari {{ data.meta.total }}</span>
        <div class="flex items-center gap-2">
          <button type="button" class="btn sm" :disabled="page <= 1" @click="page--">Sebelumnya</button>
          <span>Hal. {{ page }} / {{ totalPages }}</span>
          <button type="button" class="btn sm" :disabled="page >= totalPages" @click="page++">
            Berikutnya
          </button>
        </div>
      </div>
    </section>
  </PageState>

  <EmployeeFormModal
    :open="formOpen"
    :employee="editing"
    :departments="data?.depts ?? []"
    :people="data?.people ?? []"
    @close="formOpen = false"
    @saved="onSaved"
  />
  <ImportModal :open="importOpen" @close="importOpen = false" @imported="reload" />

  <Modal :open="!!removing" title="Hapus karyawan" @close="removing = null">
    <p class="mt-0">
      Akun <b>{{ removing?.name }}</b> akan dinonaktifkan dan tidak bisa masuk lagi. Riwayat penilaiannya
      tetap tersimpan.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="removing = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="confirm-remove"
        @click="confirmRemove"
      >
        Hapus
      </button>
    </template>
  </Modal>

  <Modal :open="!!created" title="Karyawan ditambahkan" @close="created = null">
    <p class="mt-0">
      Berikan password sementara ini kepada <b>{{ created?.name }}</b
      >. Password hanya ditampilkan sekali.
    </p>
    <p class="rounded-xl bg-canvas px-4 py-3 text-center text-xl">
      <code data-testid="temp-password">{{ created?.password }}</code>
    </p>
    <template #footer
      ><button type="button" class="btn pri" @click="created = null">Selesai</button></template
    >
  </Modal>
</template>
