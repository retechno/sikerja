<script setup lang="ts">
import { computed, ref } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import { leave, master, type CalendarItem } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useAuthStore } from '@/stores/auth'
import {
  buildMonthGrid,
  calendarKind,
  currentMonth,
  fmtRange,
  itemsOn,
  KIND_LABEL,
  monthTitle,
  shiftMonth,
  todayYmd,
} from '@/lib/leave'

// H-11 — Kalender Tim: siapa yang cuti/izin per departemen.
const auth = useAuthStore()
const month = ref(currentMonth())
const dept = ref<number>(auth.user?.department_id ?? 0) // 0 = semua
const pending = ref(true)

const WEEKDAYS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']

const { data, loading, error, reload } = useLoader(async () => {
  const [cal, departments] = await Promise.all([
    leave.calendar({
      month: month.value,
      department_id: dept.value || undefined,
      include_pending: pending.value,
    }),
    master.departments(),
  ])
  return { cal, departments }
}, [month, dept, pending])

const weeks = computed(() => buildMonthGrid(month.value))
const items = computed<CalendarItem[]>(() => data.value?.cal.items ?? [])
const holidayAt = computed(() => {
  const m = new Map<string, string>()
  for (const h of data.value?.cal.holidays ?? []) m.set(h.date.slice(0, 10), h.name)
  return m
})
const today = todayYmd()

const firstName = (n: string) => n.split(' ')[0] ?? n

function chipClass(i: CalendarItem): string {
  if (i.status === 'pending_manager' || i.status === 'pending_hr')
    return 'border-[1.5px] border-dashed border-[#D9A24A] bg-white text-[#7A4700]'
  switch (calendarKind(i.type_code)) {
    case 'sakit':
      return 'bg-[#FDE7E4] text-[#9B1C12]'
    case 'dinas':
      return 'bg-[#E3EDFA] text-[#174E8C]'
    default:
      return 'bg-[#D9ECE8] text-[#0B4A43]'
  }
}
const chipText = (i: CalendarItem) =>
  `${firstName(i.employee_name)} · ${KIND_LABEL[calendarKind(i.type_code)]}${
    i.half_day ? (i.start_time && i.end_time ? ` ${i.start_time}–${i.end_time}` : ' ½') : ''
  }`
const chipTitle = (i: CalendarItem) =>
  `${i.employee_name} — ${i.type_name}, ${fmtRange(i.start_date, i.end_date)}${
    i.status === 'approved' ? '' : ' (menunggu persetujuan)'
  }`

// daftar "tidak di kantor" bulan ini, dikelompokkan per orang
const outList = computed(() => {
  const byPerson = new Map<number, { name: string; dept: string; list: CalendarItem[] }>()
  for (const i of items.value) {
    const e = byPerson.get(i.employee_id) ?? { name: i.employee_name, dept: i.department_name, list: [] }
    e.list.push(i)
    byPerson.set(i.employee_id, e)
  }
  return [...byPerson.values()].sort((a, b) => a.name.localeCompare(b.name, 'id'))
})
</script>

<template>
  <div>
    <h2 class="m-0 text-xl font-bold">Kalender tim</h2>
    <p class="hint">Siapa yang tidak di kantor, supaya jadwal tetap aman</p>
  </div>

  <div class="flex flex-wrap items-center gap-3">
    <button
      type="button"
      class="btn !px-3"
      aria-label="Bulan sebelumnya"
      data-testid="prev"
      @click="month = shiftMonth(month, -1)"
    >
      <span class="inline-block rotate-180"><Icon name="chevron" :size="18" /></span>
    </button>
    <b class="min-w-[150px] text-center text-lg" data-testid="month-title">{{ monthTitle(month) }}</b>
    <button
      type="button"
      class="btn !px-3"
      aria-label="Bulan berikutnya"
      data-testid="next"
      @click="month = shiftMonth(month, 1)"
    >
      <Icon name="chevron" :size="18" />
    </button>
    <button type="button" class="btn" @click="month = currentMonth()">Hari ini</button>
    <span class="flex-1" />
    <label class="flex items-center gap-2 text-[13px] font-semibold"
      >Departemen
      <select
        v-model.number="dept"
        class="input !min-h-[44px] !w-auto"
        aria-label="Departemen"
        data-testid="dept"
      >
        <option v-for="d in data?.departments ?? []" :key="d.id" :value="d.id">{{ d.name }}</option>
        <option :value="0">Semua</option>
      </select>
    </label>
    <label class="flex min-h-[44px] items-center gap-2 text-[13px] font-semibold">
      <input v-model="pending" type="checkbox" class="h-[18px] w-[18px]" data-testid="pending" />Tampilkan
      yang masih menunggu
    </label>
  </div>

  <div class="flex flex-wrap gap-4 text-[12.5px]" aria-label="Legenda">
    <span class="inline-flex items-center gap-1.5"
      ><i class="inline-block h-3.5 w-3.5 rounded bg-[#D9ECE8]" />Cuti/izin disetujui</span
    >
    <span class="inline-flex items-center gap-1.5"
      ><i class="inline-block h-3.5 w-3.5 rounded border-[1.5px] border-dashed border-[#D9A24A]" />Menunggu
      persetujuan</span
    >
    <span class="inline-flex items-center gap-1.5"
      ><i class="inline-block h-3.5 w-3.5 rounded bg-[#FDE7E4]" />Sakit</span
    >
    <span class="inline-flex items-center gap-1.5"
      ><i class="inline-block h-3.5 w-3.5 rounded bg-[#E3EDFA]" />Dinas luar</span
    >
    <span class="inline-flex items-center gap-1.5"
      ><i class="inline-block h-3.5 w-3.5 rounded bg-[#F7E3B8]" />Hari libur</span
    >
  </div>

  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <div class="overflow-x-auto">
      <div
        class="grid min-w-[760px] grid-cols-7 overflow-hidden rounded-[14px] border border-line bg-white"
        role="grid"
        :aria-label="`Kalender ${monthTitle(month)}`"
        data-testid="grid"
      >
        <div
          v-for="w in WEEKDAYS"
          :key="w"
          class="border-b border-line bg-[#FAFBF9] px-3 py-2.5 text-[11.5px] font-bold uppercase tracking-wider text-ink-soft"
          role="columnheader"
        >
          {{ w }}
        </div>
        <template v-for="week in weeks" :key="week[0]!.date">
          <div
            v-for="c in week"
            :key="c.date"
            class="flex min-h-[118px] flex-col gap-1 border-b border-r border-line-soft p-2"
            :class="{ 'bg-[#F7F8F5]': c.weekend || holidayAt.has(c.date) }"
            role="gridcell"
            :data-date="c.date"
          >
            <span
              class="text-[13px] font-semibold"
              :class="[
                !c.inMonth ? 'text-[#A3ACA8]' : '',
                c.date === today
                  ? 'inline-flex h-[26px] w-[26px] items-center justify-center rounded-full bg-primary text-white'
                  : '',
              ]"
              >{{ c.day }}</span
            >
            <span
              v-if="holidayAt.get(c.date)"
              class="truncate rounded-md bg-[#F7E3B8] px-1.5 py-0.5 text-[11px] font-semibold text-[#6B4A00]"
              :title="holidayAt.get(c.date)"
              >{{ holidayAt.get(c.date) }}</span
            >
            <span
              v-for="i in itemsOn(items, c.date)"
              :key="i.id"
              class="truncate rounded-md px-1.5 py-0.5 text-[11.5px] font-semibold leading-snug"
              :class="chipClass(i)"
              :title="chipTitle(i)"
              data-testid="chip"
              >{{ chipText(i) }}</span
            >
          </div>
        </template>
      </div>
    </div>

    <section class="card" data-testid="out-list">
      <h2>Tidak di kantor bulan ini</h2>
      <p v-if="!outList.length" class="muted m-0 mt-2">Tidak ada izin atau cuti pada bulan ini.</p>
      <div v-else class="mt-3 grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-3">
        <div v-for="p in outList" :key="p.name" class="rounded-xl border border-line-soft p-3">
          <b>{{ p.name }}</b>
          <span class="muted text-xs"> · {{ p.dept }}</span>
          <div v-for="i in p.list" :key="i.id" class="mt-1 text-[13px]">
            <span class="font-semibold">{{ i.type_name }}</span>
            · {{ fmtRange(i.start_date, i.end_date)
            }}<template v-if="i.half_day && i.start_time && i.end_time"
              >, {{ i.start_time }}–{{ i.end_time }}</template
            ><template v-else-if="i.half_day && i.half_session">, {{ i.half_session }}</template>
            <span v-if="i.status !== 'approved'" class="badge b-warn ml-1 !py-0">menunggu</span>
          </div>
        </div>
      </div>
    </section>
  </PageState>
</template>
