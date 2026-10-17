<script setup lang="ts">
import type { TimelineStep } from '@/api'
import { fmtDate, fmtTime } from '@/lib/format'

defineProps<{ steps: TimelineStep[] }>()

const DOT: Record<TimelineStep['state'], string> = {
  done: 'bg-primary',
  current: 'bg-accent',
  todo: 'border-2 border-[#CBD1CA] bg-transparent',
  rejected: 'bg-bad-fg',
  cancelled: 'bg-mute-fg',
}
</script>

<template>
  <ol class="m-0 flex list-none flex-col gap-3.5 p-0" aria-label="Linimasa persetujuan">
    <li v-for="(s, i) in steps" :key="i" class="flex gap-3" :data-state="s.state">
      <span class="mt-1.5 h-3 w-3 flex-none rounded-full" :class="DOT[s.state]" aria-hidden="true" />
      <div class="min-w-0">
        <b :class="{ 'text-ink-soft': s.state === 'todo' }">{{ s.title }}</b>
        <div class="muted text-[12.5px]">
          <template v-if="s.actor">{{ s.actor }}</template>
          <template v-if="s.at"> · {{ fmtDate(s.at) }}, {{ fmtTime(s.at) }}</template>
        </div>
        <div v-if="s.note" class="mt-1 rounded-lg bg-canvas px-3 py-2 text-[13px]">“{{ s.note }}”</div>
      </div>
    </li>
  </ol>
</template>
