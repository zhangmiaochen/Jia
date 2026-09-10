<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { familyApi } from '../api'
import type { Family, MergeInvite, MergePreview, MergeSummary, PersonBrief } from '../types'

const props = defineProps<{
  visible: boolean
  family: Family | null
  myRole: string
  initialToken: string
}>()
const emit = defineEmits<{
  'update:visible': [value: boolean]
  opened: []
  merged: [summary: MergeSummary]
}>()

type Step = 'menu' | 'create' | 'redeem' | 'pair' | 'confirm' | 'done'
const KEEP = '__keep__'

const step = ref<Step>('menu')
const busy = ref(false)
const code = ref('')
const codeExpires = ref('')
const inputCode = ref('')
const invite = ref<MergeInvite | null>(null)
const sourceFamilyID = ref('')
const preview = ref<MergePreview | null>(null)
const choices = ref<Record<string, string>>({})
const mergeSameName = ref(true)
const summary = ref<MergeSummary | null>(null)

const isOwner = computed(() => props.myRole === 'owner')
const stepIndex = computed(() => ({ menu: 0, create: 1, redeem: 1, pair: 2, confirm: 3, done: 4 }[step.value]))

watch(() => props.visible, (open) => {
  if (!open) return
  reset()
  if (props.initialToken) { inputCode.value = props.initialToken; openRedeem() }
  emit('opened')
})

function reset() {
  step.value = 'menu'; busy.value = false; code.value = ''; codeExpires.value = ''
  inputCode.value = ''; invite.value = null; sourceFamilyID.value = ''
  preview.value = null; choices.value = {}; mergeSameName.value = true; summary.value = null
}
function close() { emit('update:visible', false) }

function personLabel(p: PersonBrief) {
  const birth = p.birth_date ? `（${p.birth_date}）` : ''
  const extra = [p.relation_count ? `${p.relation_count} 段关系` : '', p.genealogy_count ? `${p.genealogy_count} 份族谱` : ''].filter(Boolean).join('，')
  return `${p.name}${birth}${extra ? ` · ${extra}` : ''}`
}
function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
}

// ---------- 生成合并码 ----------
async function createCode() {
  if (!props.family || busy.value) return
  busy.value = true
  try {
    const result = await familyApi.createMergeInvite(props.family.id)
    code.value = result.token
    codeExpires.value = result.expires_at
    step.value = 'create'
  } catch (error: any) { Message.error(error.message) } finally { busy.value = false }
}
const shareLink = computed(() => `${typeof window === 'undefined' ? '' : window.location.origin}/family?merge=${code.value}`)
async function copyText(text: string, tip: string) {
  try { await navigator.clipboard.writeText(text); Message.success(tip) } catch { Message.info('复制失败，请手动选择文本复制') }
}

// ---------- 用码并入 ----------
function openRedeem() { step.value = 'redeem'; invite.value = null; preview.value = null }
const ownerFamilies = computed(() => invite.value?.my_families || [])
async function lookupCode() {
  const value = inputCode.value.trim()
  if (!value) { Message.error('请输入合并码'); return }
  busy.value = true
  try {
    const result = await familyApi.mergeInvite(value)
    invite.value = result
    const ids = result.my_families.map((f) => f.id)
    sourceFamilyID.value = ids.includes(props.family?.id || '') ? String(props.family?.id) : (ids[0] || '')
    if (!ids.length) Message.warning('你在其他家族中没有 owner 权限，无法作为被并入方')
  } catch (error: any) { invite.value = null; Message.error(error.message) } finally { busy.value = false }
}
async function runPreview() {
  if (!invite.value || !sourceFamilyID.value || busy.value) return
  busy.value = true
  try {
    const result = await familyApi.previewMerge(invite.value.token, sourceFamilyID.value)
    preview.value = result
    const next: Record<string, string> = {}
    result.suggestions.forEach((s) => { next[s.source_person_id] = s.target_person_id })
    choices.value = next
    mergeSameName.value = result.same_name_genealogies.length > 0
    step.value = 'pair'
  } catch (error: any) { Message.error(error.message) } finally { busy.value = false }
}

// ---------- 人物对照 ----------
const suggestionMap = computed(() => {
  const map: Record<string, { target_person_id: string; reason: string; confidence: string }> = {}
  preview.value?.suggestions.forEach((s) => { map[s.source_person_id] = s })
  return map
})
function optionsFor(sourcePersonID: string) {
  const used = new Set(Object.entries(choices.value).filter(([key, value]) => key !== sourcePersonID && value && value !== KEEP).map(([, value]) => value))
  return [
    { label: '保留为独立人物（不合并）', value: KEEP },
    ...(preview.value?.target_persons || []).filter((p) => !used.has(p.id)).map((p) => ({ label: personLabel(p), value: p.id })),
  ]
}
const mergedCount = computed(() => Object.values(choices.value).filter((value) => value && value !== KEEP).length)
function suggestionOf(sourcePersonID: string) {
  const hit = suggestionMap.value[sourcePersonID]
  if (!hit) return ''
  return `建议：与「${personName(hit.target_person_id)}」合并 · ${hit.reason}`
}
function personName(id: string) { return preview.value?.target_persons.find((p) => p.id === id)?.name || '' }

const pairs = computed(() => Object.entries(choices.value)
  .filter(([, target]) => target && target !== KEEP)
  .map(([source, target]) => ({ source_person_id: source, target_person_id: target })))

function gotoConfirm() {
  if (!preview.value) return
  step.value = 'confirm'
}

// ---------- 执行 ----------
async function runMerge() {
  if (!invite.value || !preview.value || busy.value) return
  busy.value = true
  try {
    const result = await familyApi.executeMerge(invite.value.token, {
      source_family_id: sourceFamilyID.value, pairs: pairs.value, merge_same_name_genealogies: mergeSameName.value,
    })
    summary.value = result
    step.value = 'done'
    Message.success('家族合并完成')
    emit('merged', result)
  } catch (error: any) { Message.error(error.message) } finally { busy.value = false }
}

const steps = ['选择方式', '合并码', '人物对照', '确认执行']
</script>

<template>
  <a-modal :visible="visible" :footer="false" :width="'min(860px, 92vw)'" :mask-closable="false" @update:visible="emit('update:visible', $event)">
    <template #title>家族合并</template>
    <div class="merge-wizard">
      <div class="merge-steps" v-if="step !== 'done'">
        <div v-for="(label, index) in steps" :key="label" class="merge-step" :class="{ active: stepIndex === index, done: stepIndex > index }">
          <span class="merge-step-no">{{ index + 1 }}</span><span>{{ label }}</span>
        </div>
      </div>

      <!-- 1. 选择方式 -->
      <div v-if="step === 'menu'" class="merge-body">
        <p class="merge-lead">如果同一个家族被几个人分别建成了多份档案，可以把其中一份整体并入另一份：人物、族谱、家谱、照片、成员都会搬过去，重复的人物可以在下一步逐条对照合并。</p>
        <div class="merge-choice">
          <div class="merge-choice-card">
            <h4>我是主家族，让别人并进来</h4>
            <p>生成一个合并码，发给对方家族的所有者。对方确认后，他的家族会整体并入当前家族（{{ family?.name || '当前家族' }}）。</p>
            <a-button type="primary" :disabled="!isOwner" :loading="busy" @click="createCode">生成合并码</a-button>
            <small v-if="!isOwner">只有当前家族的所有者（owner）可以生成合并码</small>
          </div>
          <div class="merge-choice-card">
            <h4>我要把家族并进对方</h4>
            <p>粘贴对方给你的合并码，确认后把自己管理的家族并入对方的主家族。此操作由你本人发起，需要你在这个家族里是所有者。</p>
            <a-button :disabled="!isOwner" @click="openRedeem">输入合并码</a-button>
            <small v-if="!isOwner">只有该家族的所有者（owner）可以把它并入其他家族</small>
          </div>
        </div>
        <a-alert type="warning">合并会改变大量数据，且没有一键撤销。执行前请先备份 <code>Jia_api/data/jia.db</code>。</a-alert>
      </div>

      <!-- 2a. 展示合并码 -->
      <div v-else-if="step === 'create'" class="merge-body">
        <p class="merge-lead">把下面的合并码发给对方家族的所有者（有效期 7 天，只能使用一次）。他需要在「家族资料 → 家族合并 → 输入合并码」里粘贴确认。</p>
        <div class="merge-code">{{ code }}</div>
        <div class="merge-code-actions">
          <a-button size="small" @click="copyText(code, '合并码已复制')">复制合并码</a-button>
          <a-button size="small" @click="copyText(shareLink, '链接已复制')">复制链接</a-button>
          <span class="merge-muted" v-if="codeExpires">有效期至 {{ formatDate(codeExpires) }}</span>
        </div>
        <a-alert type="info">对方确认时，系统会先给出人物对照建议，必须由他本人核对并确认后才会真正执行。</a-alert>
      </div>

      <!-- 2b. 输入合并码 -->
      <div v-else-if="step === 'redeem'" class="merge-body">
        <div class="edit-field">
          <label>合并码</label>
          <div class="merge-inline">
            <a-input v-model="inputCode" placeholder="例如 7F3K-92QD-XY4M" allow-clear @press-enter="lookupCode" />
            <a-button type="primary" :loading="busy" @click="lookupCode">校验</a-button>
          </div>
        </div>
        <template v-if="invite">
          <div class="merge-target">
            <div class="merge-target-avatar">{{ invite.target_family?.name?.slice(0, 1) || '家' }}</div>
            <div>
              <strong>{{ invite.target_family?.name }}</strong>
              <span>主家族 · {{ invite.target_family?.person_count }} 位人物 · {{ invite.target_family?.member_count }} 位成员 · {{ invite.target_family?.genealogy_count }} 份族谱</span>
            </div>
            <a-tag color="arcoblue">合并码有效</a-tag>
          </div>
          <div class="edit-field">
            <label>要并入对方的家族（你有 owner 权限的家族）</label>
            <a-select v-model="sourceFamilyID" placeholder="选择要并入的家族">
              <a-option v-for="item in ownerFamilies" :key="item.id" :value="item.id">{{ item.name }}（{{ item.person_count }} 位人物）</a-option>
            </a-select>
            <p class="edit-hint">这个家族里的全部人物、族谱、家谱、照片和成员都会并入「{{ invite.target_family?.name }}」，之后本家族不再单独存在。</p>
          </div>
          <div class="action-row">
            <a-button @click="step = 'menu'">返回</a-button>
            <a-button type="primary" :loading="busy" :disabled="!sourceFamilyID" @click="runPreview">下一步：人物对照</a-button>
          </div>
        </template>
      </div>

      <!-- 3. 人物对照 -->
      <div v-else-if="step === 'pair' && preview" class="merge-body">
        <div class="merge-counts">
          <div><span>并入方</span><strong>{{ preview.source_family.name }}</strong><small>{{ preview.source_family.person_count }} 位人物 · {{ preview.source_family.genealogy_count }} 份族谱 · {{ preview.source_family.book_count }} 部家谱</small></div>
          <div class="merge-counts-arrow">→</div>
          <div><span>接收方</span><strong>{{ preview.target_family.name }}</strong><small>{{ preview.target_family.person_count }} 位人物 · {{ preview.target_family.genealogy_count }} 份族谱 · {{ preview.target_family.book_count }} 部家谱</small></div>
        </div>
        <a-alert v-for="item in preview.warnings" :key="item" type="warning" style="margin-bottom:8px">{{ item }}</a-alert>
        <p class="merge-lead">下面是「{{ preview.source_family.name }}」的人物。同名的人如果其实是同一位祖先，请选择合并到接收方的档案；确实是两个人的，保留为独立人物。</p>
        <div class="merge-pair-list">
          <div v-for="person in preview.source_persons" :key="person.id" class="merge-pair-row">
            <div class="merge-pair-source">
              <strong>{{ person.name }}</strong>
              <small>{{ person.birth_date || '生卒不详' }}{{ person.occupation ? ` · ${person.occupation}` : '' }} · {{ person.relation_count }} 段关系</small>
            </div>
            <div class="merge-pair-arrow">→</div>
            <div class="merge-pair-target">
              <a-select v-model="choices[person.id]" size="small" allow-search placeholder="保留为独立人物">
                <a-option v-for="option in optionsFor(person.id)" :key="option.value" :value="option.value">{{ option.label }}</a-option>
              </a-select>
              <small v-if="suggestionOf(person.id)" class="merge-suggest">{{ suggestionOf(person.id) }}</small>
            </div>
          </div>
        </div>
        <a-checkbox v-if="preview.same_name_genealogies.length" v-model="mergeSameName" style="margin-top:12px">
          两边同名的族谱合并成一份（{{ preview.same_name_genealogies.map((g) => g.name).join('、') }}）
        </a-checkbox>
        <div class="action-row" style="margin-top:16px">
          <a-button @click="step = 'redeem'">返回</a-button>
          <a-button type="primary" @click="gotoConfirm">下一步：确认执行</a-button>
        </div>
      </div>

      <!-- 4. 确认 -->
      <div v-else-if="step === 'confirm' && preview" class="merge-body">
        <h4 class="merge-confirm-title">确认把「{{ preview.source_family.name }}」并入「{{ preview.target_family.name }}」？</h4>
        <ul class="merge-checklist">
          <li>{{ preview.source_family.person_count }} 位人物移入接收方，其中 <b>{{ mergedCount }}</b> 位按人物对照合并到已有档案</li>
          <li>{{ preview.source_family.genealogy_count }} 份族谱{{ mergeSameName && preview.same_name_genealogies.length ? `（${preview.same_name_genealogies.length} 份同名族谱合并）` : '' }}、{{ preview.source_family.book_count }} 部家谱、{{ preview.source_family.photo_count }} 张照片、{{ preview.source_family.relation_count }} 条关系一并迁入</li>
          <li>{{ preview.source_family.member_count }} 位成员加入接收方（原本的 owner 保留 owner 权限）；「{{ preview.source_family.name }}」不再出现在家族列表里</li>
        </ul>
        <a-alert type="error">合并不可一键撤销，请确认双方数据都已备份。执行后「{{ preview.source_family.name }}」将无法再单独访问。</a-alert>
        <div class="action-row" style="margin-top:16px">
          <a-button @click="step = 'pair'">返回修改对照</a-button>
          <a-button type="primary" status="danger" :loading="busy" @click="runMerge">确认合并</a-button>
        </div>
      </div>

      <!-- 5. 结果 -->
      <div v-else-if="step === 'done' && summary" class="merge-body">
        <div class="merge-done">
          <div class="merge-done-mark">✓</div>
          <h4>合并完成</h4>
          <p>{{ summary.source_family?.name }} → {{ summary.target_family?.name }}</p>
        </div>
        <div class="merge-stats">
          <div><strong>{{ summary.person_count }}</strong><span>人物迁入</span></div>
          <div><strong>{{ summary.persons_merged }}</strong><span>重复人物合并</span></div>
          <div><strong>{{ summary.relations_moved }}</strong><span>关系迁入</span></div>
          <div><strong>{{ summary.genealogies_moved }}</strong><span>族谱迁入</span></div>
          <div><strong>{{ summary.books_moved }}</strong><span>家谱迁入</span></div>
          <div><strong>{{ summary.photos_moved }}</strong><span>照片迁入</span></div>
          <div><strong>{{ summary.members_joined }}</strong><span>成员加入</span></div>
          <div><strong>{{ summary.members_upgraded }}</strong><span>提升为 owner</span></div>
        </div>
        <a-alert v-for="item in summary.warnings" :key="item" type="warning" style="margin-bottom:8px">{{ item }}</a-alert>
        <p class="merge-muted">全部数据已并入接收方，刷新后即可在新家族里看到。人物对照记录已保留，便于事后追溯。</p>
        <div class="action-row"><a-button type="primary" @click="close">完成</a-button></div>
      </div>
    </div>
  </a-modal>
</template>
