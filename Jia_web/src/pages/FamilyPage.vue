<script setup lang="ts">
import { ref, watch } from 'vue'
import MergeWizard from '../components/MergeWizard.vue'
import type { Family, MergeSummary } from '../types'

const props = defineProps<{ family: Family | null; myRole: string; mergeToken: string }>()
const emit = defineEmits<{ edit: []; merged: [summary: MergeSummary] }>()

const wizardVisible = ref(false)
watch(() => props.mergeToken, (value) => { if (value) wizardVisible.value = true }, { immediate: true })
</script>

<template>
  <div class="profile-grid">
    <div class="profile-card"><div class="large-avatar">{{ family?.name?.slice(0, 1) || '家' }}</div><h2>{{ family?.name || '未命名家族' }}</h2><p>{{ family?.surname ? `${family.surname}氏家族` : '数字家族档案' }}</p></div>
    <div class="panel">
      <div class="panel-title"><h3>家族资料</h3><a-button type="text" @click="emit('edit')">编辑资料</a-button></div>
      <div class="detail-list"><div><label>姓氏</label><strong>{{ family?.surname || '—' }}</strong></div><div><label>家族起源</label><strong>{{ family?.origin || '—' }}</strong></div><div><label>迁徙历史</label><strong>{{ family?.migration_history || '—' }}</strong></div><div><label>家训</label><strong>{{ family?.creed || '—' }}</strong></div></div>
      <div style="margin-top:24px"><label style="color:#8a9992;font-size:11px">家族简介</label><p style="line-height:1.8;color:#546b61;font-size:13px">{{ family?.description || '还没有家族简介。' }}</p></div>
    </div>
    <div class="panel" style="grid-column:1/-1">
      <div class="panel-title"><h3>家族合并</h3><a-tag v-if="myRole === 'owner'" color="green">你是本家族 owner</a-tag><a-tag v-else color="gray">需要 owner 权限</a-tag></div>
      <div class="merge-entry">
        <p>同一个家族被几个人分别建成了多份档案时，可以合并成一份：把另一份档案里的人物、族谱、家谱、照片和成员整体并入本家族，重复的人物在合并前逐条对照确认。合并需要双方家族的所有者共同确认。</p>
        <a-button type="primary" :disabled="!family" @click="wizardVisible = true">开始合并</a-button>
      </div>
    </div>
  </div>
  <MergeWizard v-model:visible="wizardVisible" :family="family" :my-role="myRole" :initial-token="mergeToken" @merged="emit('merged', $event)" />
</template>
