<script setup lang="ts">
import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { genealogyApi, personApi } from '../api'
import type { Genealogy, Person } from '../types'
const props = defineProps<{ genealogies: Genealogy[]; familyId?: string; formatTime: (value: string) => string }>()
const emit = defineEmits<{ create: []; refresh: [] }>()
const selected = ref<Genealogy | null>(null)
const form = ref({ name: '', description: '' })
const linked = ref<Person[]>([])
const people = ref<Person[]>([])
const personID = ref('')
const busy = ref(false)
const createVisible = ref(false)
const createForm = ref({ name: '', description: '' })
function startCreate() { createForm.value = { name: '', description: '' }; createVisible.value = true }
async function create() { if (busy.value) return; if (!props.familyId) { Message.error('请先选择家族'); return } if (!createForm.value.name.trim()) { Message.error('请输入族谱名称'); return } busy.value = true; try { await genealogyApi.create(props.familyId, createForm.value); createVisible.value = false; emit('refresh'); Message.success('族谱已创建') } catch (e: any) { Message.error(e.message) } finally { busy.value = false } }
async function open(g: Genealogy) { try { const [detail, members, all] = await Promise.all([genealogyApi.get(g.id), genealogyApi.persons(g.id), personApi.list(g.family_id)]); form.value = { name: detail.name, description: detail.description }; linked.value = members; people.value = all; personID.value = ''; selected.value = detail } catch (e: any) { Message.error(e.message) } }
async function save() { if (!selected.value || busy.value) return; if (!form.value.name.trim()) { Message.error('请输入族谱名称'); return } busy.value = true; try { await genealogyApi.update(selected.value.id, form.value); emit('refresh'); Message.success('族谱已保存') } catch (e: any) { Message.error(e.message) } finally { busy.value = false } }
async function associate(id: string, remove = false) { if (!selected.value || !id || busy.value) return; busy.value = true; try { if (remove) await genealogyApi.removePerson(selected.value.id, id); else await genealogyApi.addPerson(selected.value.id, id); linked.value = await genealogyApi.persons(selected.value.id); personID.value = ''; emit('refresh') } catch (e: any) { Message.error(e.message) } finally { busy.value = false } }
async function remove(g: Genealogy) { try { await genealogyApi.remove(g.id); selected.value = null; emit('refresh'); Message.success('族谱已删除') } catch (e: any) { Message.error(e.message) } }
</script>
<template>
  <div class="section-head"><h2>族谱分支</h2><a-button type="primary" @click="startCreate">新建族谱</a-button></div>
  <div class="member-list"><div v-for="g in props.genealogies" :key="g.id" class="member"><div><strong>{{ g.name }}</strong><p>{{ g.person_count || 0 }} 位人物 · {{ formatTime(g.updated_at) }}</p></div><div class="action-row"><a-button @click="open(g)">详情与编辑</a-button><a-popconfirm content="确定删除此族谱？人物档案会保留。" @ok="remove(g)"><a-button status="danger">删除</a-button></a-popconfirm></div></div></div>
  <a-empty v-if="!genealogies.length" />
  <a-modal :visible="!!selected" title="族谱资料与人物" :width="680" :footer="false" @cancel="selected = null">
    <a-form :model="form" layout="vertical"><a-form-item label="族谱名称" required><a-input v-model="form.name" /></a-form-item><a-form-item label="说明"><a-textarea v-model="form.description" /></a-form-item><a-button :loading="busy" type="primary" @click="save">保存资料</a-button></a-form>
    <h3>关联人物（{{ linked.length }}）</h3><div class="action-row"><a-select v-model="personID" allow-search placeholder="选择人物" style="min-width:180px;flex:1"><a-option v-for="p in people.filter(p => !linked.some(l => l.id === p.id))" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select><a-button :disabled="!personID || busy" @click="associate(personID)">添加关联</a-button></div>
    <div class="linked-scroll"><div v-for="p in linked" :key="p.id" class="member"><span>{{ p.name }}</span><a-popconfirm content="从族谱中移除此人物？" @ok="associate(p.id, true)"><a-button :disabled="busy" type="text" status="danger">移除关联</a-button></a-popconfirm></div><div v-if="!linked.length" class="empty">暂无关联人物</div></div>
  </a-modal>
  <a-modal v-model:visible="createVisible" title="新建族谱" :footer="false"><a-form :model="createForm" layout="vertical"><a-form-item label="族谱名称" required><a-input v-model="createForm.name" /></a-form-item><a-form-item label="说明"><a-textarea v-model="createForm.description" /></a-form-item><div class="action-row"><a-button @click="createVisible = false">取消</a-button><a-button type="primary" :loading="busy" @click="create">确定创建</a-button></div></a-form></a-modal>
</template>
