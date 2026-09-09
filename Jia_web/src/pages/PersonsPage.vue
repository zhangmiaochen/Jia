<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { personApi } from '../api'
import type { Book, Family, Genealogy, Person, PersonFamily, Photo } from '../types'
const props = defineProps<{ persons: Person[]; query: string; genderLabel: (gender: string) => string; allFamilies: Family[] }>()
const emit = defineEmits<{ 'update:query': [value: string]; refresh: []; create: []; graph: [person: Person]; move: [person: Person]; family: [person: Person] }>()
const selected = ref<Person | null>(null)
const detail = ref<{ families: PersonFamily[]; genealogies: Genealogy[]; books: Book[]; photos: Photo[] } | null>(null)
const busy = ref(false)
async function open(p: Person) { try { selected.value = await personApi.get(p.id); detail.value = null; const [families, genealogies, books, photos] = await Promise.all([personApi.families(p.id), personApi.genealogies(p.id), personApi.books(p.id), personApi.photos(p.id)]); detail.value = { families, genealogies, books, photos } } catch (e: any) { Message.error(e.message) } }
async function save() { const p = selected.value; if (!p || busy.value) return; if (!p.name.trim()) { Message.error('请输入姓名'); return } busy.value = true; try { await personApi.update(p.id, { name: p.name, gender: p.gender, birthDate: p.birth_date, deathDate: p.death_date, birthplace: p.birthplace, occupation: p.occupation, biography: p.biography }); selected.value = null; emit('refresh'); Message.success('人物资料已保存') } catch (e: any) { Message.error(e.message) } finally { busy.value = false } }
async function remove(p: Person) { try { await personApi.remove(p.id); selected.value = null; emit('refresh'); Message.success('人物已删除') } catch (e: any) { Message.error(e.message) } }
async function claim(p: Person) { try { await personApi.claim(p.id); selected.value = await personApi.get(p.id); emit('refresh'); Message.success('已认领') } catch (e: any) { Message.error(e.message) } }

// ---- 人物列表：固定行高 + 虚拟滚动（人数再多页面也不会拉长） ----
const ROW_H = 56
const VIEW_H = 560
const scrollerTop = ref(0)
const personViewHeight = computed(() => Math.min(VIEW_H, Math.max(ROW_H, props.persons.length * ROW_H)))
const visiblePersons = computed(() => {
  const total = props.persons.length
  const start = Math.max(0, Math.floor(scrollerTop.value / ROW_H) - 4)
  const end = Math.min(total, Math.ceil((scrollerTop.value + personViewHeight.value) / ROW_H) + 4)
  return props.persons.slice(start, end)
})
const personRowTop = computed(() => Math.floor(scrollerTop.value / ROW_H) * ROW_H)
function onPersonScroll(event: Event) { scrollerTop.value = (event.target as HTMLElement).scrollTop }
watch(() => props.persons.length, () => { scrollerTop.value = 0 })
</script>
<template>
  <div class="section-head"><h2>人物索引</h2><a-button type="primary" @click="emit('create')">添加人物</a-button></div>
  <div class="toolbar"><input :value="query" class="search" placeholder="搜索姓名" @input="emit('update:query', ($event.target as HTMLInputElement).value)"><a-button @click="emit('refresh')">刷新资料</a-button></div>
  <div class="person-list-wrap">
    <div class="person-list-head"><span>人物</span><span class="col-gender">性别</span><span class="col-birth">出生</span><span class="col-status">状态</span><span style="text-align:right">操作</span></div>
    <div v-if="persons.length" class="person-list-viewport" :style="{ height: personViewHeight + 'px' }" @scroll.passive="onPersonScroll">
      <div class="person-list-content" :style="{ height: persons.length * ROW_H + 'px' }">
        <div class="person-list-window" :style="{ transform: `translateY(${personRowTop}px)` }">
          <div v-for="p in visiblePersons" :key="p.id" class="person-row">
            <span class="cell-name">{{ p.name }}</span><span class="cell-plain cell-gender">{{ genderLabel(p.gender) }}</span><span class="cell-plain cell-birth">{{ p.birth_date }}</span><span class="cell-status">{{ p.claimed_by ? '已认领' : '待认领' }}</span>
            <span class="cell-actions"><a-button size="small" @click="open(p)">详情与编辑</a-button><a-button size="small" @click="emit('graph', p)">关系</a-button><a-button size="small" @click="emit('family', p)">家族</a-button><a-button size="small" class="btn-move" @click="emit('move', p)">移入家族</a-button><a-popconfirm content="确定删除人物档案？" @ok="remove(p)"><a-button size="small" status="danger">删除</a-button></a-popconfirm></span>
          </div>
        </div>
      </div>
    </div>
    <a-empty v-else style="padding: 30px 0" description="暂无人物" />
  </div>
  <a-modal :visible="!!selected" title="人物资料" :footer="false" @cancel="selected = null"><a-form v-if="selected" :model="selected" layout="vertical">
    <a-form-item label="姓名" required><a-input v-model="selected.name" /></a-form-item><a-form-item label="性别"><a-select v-model="selected.gender"><a-option value="male">男</a-option><a-option value="female">女</a-option><a-option value="unknown">未填写</a-option></a-select></a-form-item>
    <a-form-item label="出生日期"><a-input v-model="selected.birth_date" placeholder="YYYY-MM-DD" /></a-form-item><a-form-item label="去世日期"><a-input v-model="selected.death_date" placeholder="YYYY-MM-DD" /></a-form-item><a-form-item label="籍贯"><a-input v-model="selected.birthplace" /></a-form-item><a-form-item label="职业"><a-input v-model="selected.occupation" /></a-form-item><a-form-item label="简介"><a-textarea v-model="selected.biography" /></a-form-item>
    <div class="action-row"><a-button type="primary" :loading="busy" @click="save">保存</a-button><a-popconfirm v-if="!selected.claimed_by" content="确认这是你本人的档案？" @ok="claim(selected!)"><a-button>认领本人档案</a-button></a-popconfirm></div>
    <div v-if="detail" class="detail-sections">
      <div class="detail-section"><label>所属家族</label><p>{{ detail.families.map((f) => f.name + (f.home ? '（主家族）' : '')).join('、') || '—' }}</p></div>
      <div class="detail-section"><label>关联族谱（{{ detail.genealogies.length }}）</label><p v-if="detail.genealogies.length">{{ detail.genealogies.map((g) => `${g.name}（${(allFamilies.find((f) => f.id === g.family_id)?.name || '未知家族')}）`).join('；') }}</p><p v-else>暂无</p></div>
      <div class="detail-section"><label>相关家谱（{{ detail.books.length }}）</label><p v-if="detail.books.length">{{ detail.books.map((b) => `${b.title}（${(allFamilies.find((f) => f.id === b.family_id)?.name || '未知家族')}）`).join('；') }}</p><p v-else>暂无</p></div>
      <div class="detail-section"><label>相关照片（{{ detail.photos.length }}）</label><p v-if="detail.photos.length">{{ detail.photos.map((ph) => ph.caption || ph.original_name).join('；') }}</p><p v-else>暂无</p></div>
    </div>
  </a-form></a-modal>
</template>
