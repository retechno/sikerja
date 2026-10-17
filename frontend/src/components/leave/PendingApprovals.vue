<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Avatar from '@/components/ui/Avatar.vue'
import type { LeaveRequest } from '@/api'
import { fmtDays, leaveRangeText, leaveTitle, relativeDay } from '@/lib/leave'

// Daftar singkat pengajuan yang menunggu keputusan pengguna (dasbor manajer & direktur).
defineProps<{ items: LeaveRequest[]; total: number }>()
</script>

<template>
  <section class="card min-w-0" data-testid="pending-leave">
    <div class="mb-3 flex items-start gap-3">
      <div class="flex-1">
        <h2>Persetujuan izin &amp; cuti</h2>
        <p class="hint">{{ total }} pengajuan menunggu keputusan Anda</p>
      </div>
      <RouterLink to="/persetujuan" class="btn sm pri">Buka antrean</RouterLink>
    </div>
    <p v-if="!items.length" class="muted m-0">Tidak ada pengajuan yang menunggu.</p>
    <RouterLink
      v-for="l in items"
      :key="l.id"
      :to="{ path: '/persetujuan', query: { id: String(l.id) } }"
      class="flex items-center gap-3 border-b border-line-soft py-2.5 text-ink no-underline last:border-b-0 hover:text-ink"
    >
      <Avatar :employee="l.employee" size="s" />
      <span class="min-w-0 flex-1">
        <b class="block truncate">{{ l.employee?.name }}</b>
        <span class="muted block truncate text-xs"
          >{{ leaveTitle(l) }} · {{ leaveRangeText(l) }} · {{ fmtDays(l.days) }} hari</span
        >
      </span>
      <span class="muted text-xs">{{ relativeDay(l.created_at) }}</span>
    </RouterLink>
  </section>
</template>
