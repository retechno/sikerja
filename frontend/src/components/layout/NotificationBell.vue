<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/ui/Icon.vue'
import { notifications, type NotificationItem } from '@/api'
import { fmtDate } from '@/lib/format'

const router = useRouter()
const open = ref(false)
const items = ref<NotificationItem[]>([])
const unread = ref(0)

async function load() {
  try {
    const r = await notifications.list()
    items.value = r.items ?? []
    unread.value = r.unread
  } catch {
    /* lonceng tidak boleh mengganggu halaman */
  }
}

async function readAll() {
  await notifications.readAll()
  await load()
}

interface Parsed {
  title?: string
  message?: string
  link?: string
}

function parse(n: NotificationItem): Parsed {
  try {
    return JSON.parse(n.data) as Parsed
  } catch {
    return {}
  }
}
const label = (n: NotificationItem) => parse(n).message ?? n.type

/** Buka tautan notifikasi dan tandai terbaca. */
async function openItem(n: NotificationItem) {
  const link = parse(n).link
  open.value = false
  if (!n.read_at) {
    try {
      await notifications.read(n.id)
    } catch {
      /* abaikan */
    }
    void load()
  }
  if (link) await router.push(link)
}

let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void load()
  timer = setInterval(load, 60_000) // lonceng menyegarkan diri tiap menit
})
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <div class="relative">
    <button
      type="button"
      class="relative grid h-11 w-11 place-items-center rounded-xl border border-line bg-white"
      aria-label="Notifikasi"
      :aria-expanded="open"
      @click="open = !open"
    >
      <Icon name="bell" :size="20" />
      <span
        v-if="unread > 0"
        class="absolute right-2.5 top-2.5 h-2 w-2 rounded-full border-2 border-white bg-[#C2410C]"
      />
    </button>
    <div
      v-if="open"
      class="absolute right-0 top-[52px] z-30 w-[min(20rem,calc(100vw-2rem))] rounded-xl border border-line bg-white shadow-lg"
    >
      <div class="flex items-center justify-between border-b border-line px-4 py-3">
        <b>Notifikasi</b>
        <button v-if="unread > 0" type="button" class="text-xs font-semibold text-primary" @click="readAll">
          Tandai terbaca
        </button>
      </div>
      <p v-if="!items.length" class="m-0 px-4 py-6 text-center text-ink-soft">Belum ada notifikasi.</p>
      <ul v-else class="m-0 max-h-72 list-none overflow-y-auto p-0">
        <li v-for="n in items" :key="n.id" class="border-b border-line-soft last:border-0">
          <button
            type="button"
            class="block w-full px-4 py-3 text-left hover:bg-canvas"
            :class="{ 'bg-primary-tint': !n.read_at }"
            :data-testid="`notif-${n.id}`"
            @click="openItem(n)"
          >
            <b v-if="parse(n).title" class="block text-[13px]">{{ parse(n).title }}</b>
            <span class="text-[13px]">{{ label(n) }}</span>
            <span class="muted mt-0.5 block text-[11.5px]">{{ fmtDate(n.created_at) }}</span>
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
