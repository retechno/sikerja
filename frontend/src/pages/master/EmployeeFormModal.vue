<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import Modal from '@/components/ui/Modal.vue'
import { ApiError, employees, master, type Department, type Employee, type Position } from '@/api'
import { ROLE_LABEL, toDateInput } from '@/lib/format'

// Form tambah / ubah karyawan (H-12). Role, atasan, dan status diatur di sini.
const props = defineProps<{
  open: boolean
  employee: Employee | null
  departments: Department[]
  people: Employee[]
}>()
const emit = defineEmits<{ close: []; saved: [tempPassword: string, name: string] }>()

const ROLES = ['employee', 'manager', 'hr', 'director'] as const

const schema = toTypedSchema(
  z
    .object({
      nik: z.string().trim().min(1, 'NIK wajib diisi').max(30, 'Maksimal 30 karakter'),
      name: z.string().trim().min(1, 'Nama wajib diisi').max(150, 'Maksimal 150 karakter'),
      email: z.string().trim().min(1, 'Email wajib diisi').email('Format email tidak valid'),
      phone: z.string().trim().max(30, 'Maksimal 30 karakter').default(''),
      department_id: z.number().nullable(),
      position_id: z.number().nullable(),
      manager_id: z.number().nullable(),
      join_date: z.string().min(1, 'Tanggal masuk wajib diisi'),
      status: z.enum(['active', 'inactive']),
      roles: z.array(z.string()).min(1, 'Pilih minimal satu role'),
      password: z.string().default(''),
    })
    .refine(
      (v) => !(v.roles.includes('employee') || v.roles.includes('manager')) || v.department_id !== null,
      {
        path: ['department_id'],
        message: 'Departemen wajib diisi untuk karyawan dan manajer',
      },
    )
    .refine((v) => v.password === '' || v.password.length >= 8, {
      path: ['password'],
      message: 'Minimal 8 karakter',
    }),
)

const { defineField, handleSubmit, errors, resetForm, setErrors, setFieldValue, values } = useForm({
  validationSchema: schema,
})
const [nik] = defineField('nik')
const [name] = defineField('name')
const [email] = defineField('email')
const [phone] = defineField('phone')
const [departmentId] = defineField('department_id')
const [positionId] = defineField('position_id')
const [managerId] = defineField('manager_id')
const [joinDate] = defineField('join_date')
const [status] = defineField('status')
const [roles] = defineField('roles')
const [password] = defineField('password')

const positions = ref<Position[]>([])
const busy = ref(false)
const formError = ref('')
const editing = computed(() => props.employee !== null)

watch(
  () => props.open,
  (o) => {
    if (!o) return
    const e = props.employee
    formError.value = ''
    resetForm({
      values: {
        nik: e?.nik ?? '',
        name: e?.name ?? '',
        email: e?.email ?? '',
        phone: e?.phone ?? '',
        department_id: e?.department_id ?? null,
        position_id: e?.position_id ?? null,
        manager_id: e?.manager_id ?? null,
        join_date: e ? toDateInput(e.join_date) : toDateInput(new Date().toISOString()),
        status: e?.status ?? 'active',
        roles: e?.roles?.length ? [...e.roles] : ['employee'],
        password: '',
      },
    })
  },
)

// jabatan mengikuti departemen
watch(
  () => values.department_id,
  async (d) => {
    positions.value = d ? await master.positions(d).catch(() => []) : await master.positions().catch(() => [])
    if (positionId.value && !positions.value.some((p) => p.id === positionId.value))
      setFieldValue('position_id', null)
  },
  { immediate: true },
)

const managerOptions = computed(() =>
  props.people.filter((p) => p.status === 'active' && p.id !== props.employee?.id),
)

function toggleRole(r: string) {
  const cur = new Set(roles.value ?? [])
  if (!cur.delete(r)) cur.add(r)
  roles.value = [...cur]
}

const num = (v: string) => (v === '' ? null : Number(v))

const submit = handleSubmit(async (v) => {
  busy.value = true
  formError.value = ''
  const body = { ...v, phone: v.phone ?? '', password: v.password || undefined }
  try {
    if (props.employee) {
      await employees.update(props.employee.id, body)
      emit('saved', '', v.name)
    } else {
      const r = await employees.create(body)
      emit('saved', r.tempPassword, v.name)
    }
  } catch (e) {
    if (e instanceof ApiError) {
      formError.value = e.message
      if (Object.keys(e.fields).length) setErrors(e.fields)
    } else {
      formError.value = 'Gagal menyimpan'
    }
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <Modal :open="open" :title="editing ? 'Ubah karyawan' : 'Tambah karyawan'" wide @close="emit('close')">
    <form id="employee-form" class="grid gap-4 sm:grid-cols-2" novalidate @submit.prevent="submit">
      <div v-if="formError" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg sm:col-span-2" role="alert">
        {{ formError }}
      </div>

      <label class="field"
        >NIK
        <input v-model="nik" class="input" :class="{ invalid: errors.nik }" data-testid="f-nik" />
        <span v-if="errors.nik" class="err">{{ errors.nik }}</span>
      </label>
      <label class="field"
        >Nama lengkap
        <input v-model="name" class="input" :class="{ invalid: errors.name }" data-testid="f-name" />
        <span v-if="errors.name" class="err">{{ errors.name }}</span>
      </label>
      <label class="field"
        >Email
        <input
          v-model="email"
          type="email"
          class="input"
          :class="{ invalid: errors.email }"
          data-testid="f-email"
        />
        <span v-if="errors.email" class="err">{{ errors.email }}</span>
      </label>
      <label class="field"
        >Telepon
        <input v-model="phone" class="input" :class="{ invalid: errors.phone }" />
        <span v-if="errors.phone" class="err">{{ errors.phone }}</span>
      </label>

      <label class="field"
        >Departemen
        <select
          class="input"
          :value="departmentId ?? ''"
          :class="{ invalid: errors.department_id }"
          data-testid="f-dept"
          @change="departmentId = num(($event.target as HTMLSelectElement).value)"
        >
          <option value="">— tanpa departemen —</option>
          <option v-for="d in departments" :key="d.id" :value="d.id">{{ d.name }}</option>
        </select>
        <span v-if="errors.department_id" class="err">{{ errors.department_id }}</span>
      </label>
      <label class="field"
        >Jabatan
        <select
          class="input"
          :value="positionId ?? ''"
          @change="positionId = num(($event.target as HTMLSelectElement).value)"
        >
          <option value="">— pilih jabatan —</option>
          <option v-for="p in positions" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>

      <label class="field"
        >Atasan langsung
        <select
          class="input"
          :value="managerId ?? ''"
          @change="managerId = num(($event.target as HTMLSelectElement).value)"
        >
          <option value="">— tanpa atasan —</option>
          <option v-for="p in managerOptions" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>
      <label class="field"
        >Tanggal masuk
        <input
          v-model="joinDate"
          type="date"
          class="input"
          :class="{ invalid: errors.join_date }"
          data-testid="f-join"
        />
        <span v-if="errors.join_date" class="err">{{ errors.join_date }}</span>
      </label>

      <fieldset class="field m-0 border-0 p-0 sm:col-span-2">
        <legend class="mb-1.5 p-0">Role pengguna</legend>
        <div class="flex flex-wrap gap-x-5 gap-y-1">
          <label v-for="r in ROLES" :key="r" class="flex min-h-[44px] items-center gap-2 font-medium">
            <input
              type="checkbox"
              class="h-[18px] w-[18px]"
              :checked="roles?.includes(r)"
              :data-testid="`f-role-${r}`"
              @change="toggleRole(r)"
            />
            {{ ROLE_LABEL[r] }}
          </label>
        </div>
        <small>Satu orang boleh punya lebih dari satu role (mis. Manajer yang juga Karyawan).</small>
        <span v-if="errors.roles" class="err">{{ errors.roles }}</span>
      </fieldset>

      <label v-if="editing" class="field"
        >Status
        <select v-model="status" class="input">
          <option value="active">Aktif</option>
          <option value="inactive">Nonaktif (tidak bisa masuk)</option>
        </select>
      </label>
      <label v-else class="field"
        >Password awal <small>(opsional)</small>
        <input
          v-model="password"
          type="text"
          autocomplete="off"
          class="input"
          :class="{ invalid: errors.password }"
          placeholder="Kosong = dibuat acak"
        />
        <small
          >Jika kosong, sistem membuat password sementara dan menampilkannya sekali setelah disimpan.</small
        >
        <span v-if="errors.password" class="err">{{ errors.password }}</span>
      </label>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="emit('close')">Batal</button>
      <button type="submit" form="employee-form" class="btn pri" :disabled="busy" data-testid="f-save">
        {{ busy ? 'Menyimpan…' : 'Simpan' }}
      </button>
    </template>
  </Modal>
</template>
