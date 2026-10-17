<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Modal from '@/components/ui/Modal.vue'
import PageState from '@/components/ui/PageState.vue'
import { ApiError, leave, type HolidayCandidate } from '@/api'
import { fmtWeekdayDate } from '@/lib/leave'
import { useToastStore } from '@/stores/toast'

// H-16 (tab "Hari libur"): impor dari kalender eksternal. Selalu lewat pratinjau — HR memeriksa terhadap SKB
// dulu, baru menyimpan. Tanggal yang sudah ada tidak bisa dipilih (tidak akan ditimpa).
const props = defineProps<{ open: boolean; year: number }>()
const emit = defineEmits<{ close: []; done: []; builtin: [] }>()
const toast = useToastStore()

const items = ref<(HolidayCandidate & { picked: boolean })[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  items.value = []
  try {
    const list = await leave.previewHolidayImport(props.year)
    items.value = list.map((c) => ({ ...c, picked: !c.exists && !c.tentative }))
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Gagal mengambil daftar hari libur'
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.open, props.year] as const,
  ([o]) => {
    if (o) void load()
  },
  { immediate: true },
)

const selectable = computed(() => items.value.filter((i) => !i.exists))
const picked = computed(() => items.value.filter((i) => i.picked && !i.exists))
const allPicked = computed(
  () => selectable.value.length > 0 && picked.value.length === selectable.value.length,
)
const tentative = computed(() => picked.value.filter((i) => i.tentative).length)

function toggleAll() {
  const v = !allPicked.value
  for (const i of selectable.value) i.picked = v
}

async function save() {
  if (!picked.value.length) return
  busy.value = true
  try {
    const r = await leave.confirmHolidayImport(
      picked.value.map((i) => ({ date: i.date, name: i.name, type: i.type })),
    )
    toast.success(`${r.added} hari libur ditambahkan.`)
    emit('done')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menyimpan')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :open="open" :title="`Impor hari libur ${year}`" wide @close="emit('close')">
    <div class="mb-3 rounded-xl bg-warn-bg px-4 py-3 text-warn-fg" role="note">
      Daftar ini diambil dari kalender publik, bukan dari SKB Tiga Menteri. Cocokkan dengan SKB resmi sebelum
      menyimpan — tanggal cuti bersama bisa berubah.
    </div>
    <PageState
      :loading="loading"
      :error="error"
      :empty="!loading && !error && !items.length"
      :empty-text="`Sumber belum memuat hari libur untuk ${year}. Coba lagi nanti atau tambahkan manual setelah SKB terbit.`"
      @retry="load"
    >
      <label
        class="flex min-h-[44px] items-center gap-3 border-b border-line-soft py-2 font-semibold"
        data-testid="pick-all"
      >
        <input
          type="checkbox"
          class="h-[18px] w-[18px]"
          :checked="allPicked"
          :disabled="!selectable.length"
          @change="toggleAll"
        />Pilih semua yang belum ada ({{ selectable.length }})
      </label>
      <label
        v-for="i in items"
        :key="i.date"
        class="flex min-h-[44px] items-center gap-3 border-b border-line-soft py-2 last:border-b-0"
        :class="{ 'opacity-60': i.exists }"
        :data-testid="`cand-${i.date}`"
      >
        <input v-model="i.picked" type="checkbox" class="h-[18px] w-[18px] flex-none" :disabled="i.exists" />
        <span class="min-w-0 flex-1">
          <b class="block">{{ i.name }}</b>
          <span class="muted text-[12.5px]"
            >{{ fmtWeekdayDate(i.date)
            }}<template v-if="i.exists"> · sudah ada: {{ i.existing_name }}</template></span
          >
        </span>
        <span v-if="i.tentative" class="badge b-warn">Tentatif</span>
        <span v-if="i.exists" class="badge b-mute">Sudah ada</span>
        <span v-else class="badge" :class="i.type === 'nasional' ? 'b-info' : 'b-warn'">{{
          i.type === 'nasional' ? 'Nasional' : 'Cuti bersama'
        }}</span>
      </label>
    </PageState>
    <p v-if="tentative" class="hint mb-0 mt-3" data-testid="tentative-note">
      {{ tentative }} tanggal yang dipilih ditandai tentatif oleh sumber.
    </p>
    <template #footer>
      <button
        type="button"
        class="btn mr-auto"
        data-testid="import-builtin"
        :disabled="busy"
        @click="emit('builtin')"
      >
        Pakai daftar bawaan
      </button>
      <button type="button" class="btn" @click="emit('close')">Batal</button>
      <button
        type="button"
        class="btn pri"
        :disabled="busy || !picked.length"
        data-testid="import-confirm"
        @click="save"
      >
        Simpan {{ picked.length }} terpilih
      </button>
    </template>
  </Modal>
</template>
