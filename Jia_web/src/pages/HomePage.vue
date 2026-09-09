<script setup lang="ts">
import type { Activity, Family } from '../types'
defineProps<{ family: Family | null; personsCount: number; genealogiesCount: number; booksCount: number; activities: Activity[]; formatTime: (value: string) => string }>()
const emit = defineEmits<{ navigate: [key: string] }>()
</script>

<template>
  <div class="stat-grid"><div class="stat-card"><span>档案人物</span><strong>{{ family?.person_count || personsCount }}</strong></div><div class="stat-card"><span>族谱分支</span><strong>{{ family?.genealogy_count || genealogiesCount }}</strong></div><div class="stat-card"><span>家族成员</span><strong>{{ family?.member_count || '—' }}</strong></div><div class="stat-card"><span>家谱记录</span><strong>{{ booksCount }}</strong></div></div>
  <div class="workspace-grid"><div class="panel"><div class="panel-title"><h3>当前家族</h3><a @click="emit('navigate', 'family')">查看资料 →</a></div><div v-if="family" class="family-card"><div><h3>{{ family.name }}</h3><p>{{ family.surname ? `${family.surname}氏 · ` : '' }}{{ family.origin || '尚未填写家族起源' }}</p></div><div class="metrics"><div><strong>{{ personsCount }}</strong><span>人物</span></div><div><strong>{{ genealogiesCount }}</strong><span>族谱</span></div><div><strong>{{ booksCount }}</strong><span>家谱</span></div></div></div><div v-else class="empty">还没有家族档案，先创建一个家族吧。</div></div><div class="panel"><div class="panel-title"><h3>最近动态</h3><a @click="emit('navigate', 'activity')">全部动态 →</a></div><div v-if="activities.length" class="activity-list"><div v-for="item in activities.slice(0, 5)" :key="item.id" class="activity-item"><div class="activity-dot">✦</div><p>{{ item.summary }}</p><time>{{ formatTime(item.created_at) }}</time></div></div><div v-else class="empty">家族的第一条动态，等待你来书写。</div></div></div>
</template>
