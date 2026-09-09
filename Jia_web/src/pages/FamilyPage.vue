<script setup lang="ts">
import type { Family } from '../types'
defineProps<{ family: Family | null }>()
const emit = defineEmits<{ edit: [] }>()
</script>

<template>
  <div class="profile-grid"><div class="profile-card"><div class="large-avatar">{{ family?.name?.slice(0, 1) || '家' }}</div><h2>{{ family?.name || '未命名家族' }}</h2><p>{{ family?.surname ? `${family.surname}氏家族` : '数字家族档案' }}</p></div><div class="panel"><div class="panel-title"><h3>家族资料</h3><a-button type="text" @click="emit('edit')">编辑资料</a-button></div><div class="detail-list"><div><label>姓氏</label><strong>{{ family?.surname || '—' }}</strong></div><div><label>家族起源</label><strong>{{ family?.origin || '—' }}</strong></div><div><label>迁徙历史</label><strong>{{ family?.migration_history || '—' }}</strong></div><div><label>家训</label><strong>{{ family?.creed || '—' }}</strong></div></div><div style="margin-top:24px"><label style="color:#8a9992;font-size:11px">家族简介</label><p style="line-height:1.8;color:#546b61;font-size:13px">{{ family?.description || '还没有家族简介。' }}</p></div></div></div>
</template>
