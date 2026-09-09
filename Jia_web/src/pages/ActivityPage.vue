<script setup lang="ts">
import type { Activity } from '../types'
defineProps<{ activities: Activity[]; formatTime: (value: string) => string }>()
</script>

<template>
  <div class="panel" style="margin-top:30px"><div class="activity-list"><div v-for="item in activities" :key="item.id" class="activity-item"><div class="activity-dot">✦</div><p>{{ item.summary }}</p><time>{{ formatTime(item.created_at) }}</time></div><div v-if="!activities.length" class="empty">暂无动态。</div></div></div>
</template>
