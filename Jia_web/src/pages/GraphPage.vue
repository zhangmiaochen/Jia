<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { Graph, register, ExtensionCategory, BaseLayout } from '@antv/g6'
import type { GraphData as G6GraphData } from '@antv/g6'
import { personApi } from '../api'
import type { GraphData, Person, PersonFamily, Relation } from '../types'
import { layoutFamilyGraph, pairKey } from '../utils/graphLayout'
import type { DrawnEdge } from '../utils/graphLayout'
import { computeKinship } from '../utils/kinship'

// 自定义布局：把外部算好的坐标（辈分分层结果）写入布局结果，走 G6 标准布局管线
class FamilyPositionsLayout extends BaseLayout {
  id = 'family-positions'
  async execute(model: G6GraphData): Promise<G6GraphData> {
    const positions = ((this.options as any)?.positions ?? {}) as Record<string, [number, number]>
    return {
      ...model,
      nodes: (model.nodes ?? []).map((n) => {
        const p = positions[String(n.id)]
        return p ? { ...n, style: { ...(n.style ?? {}), x: p[0], y: p[1] } } : n
      }),
    }
  }
}
try { register(ExtensionCategory.LAYOUT, 'family-positions', FamilyPositionsLayout as any) } catch { /* 已注册 */ }

const props = defineProps<{ persons: Person[]; graph: GraphData | null; selectedPerson: Person | null; selectedPersonId: string; personName: (id: string) => string; personFamilies?: PersonFamily[]; familyId?: string }>()
const emit = defineEmits<{ change: [id: string]; sync: [id: string]; add: []; refresh: []; tree: [familyId: string] }>()

const stageRef = ref<HTMLElement | null>(null)
const graphRef = shallowRef<Graph | null>(null)
const zoomLabel = ref('100%')

const relationNames: Record<string, string> = { FATHER: '父亲', MOTHER: '母亲', SON: '儿子', DAUGHTER: '女儿', HUSBAND: '丈夫', WIFE: '妻子', BROTHER: '兄弟', SISTER: '姐妹', GRANDFATHER: '祖父', GRANDMOTHER: '祖母', GRANDCHILD: '孙辈', UNCLE: '叔伯', AUNT: '姑姨', NEPHEW_NIECE: '侄辈' }
const relationText = (edge: Pick<Relation, 'custom_name' | 'relation_type'>) => edge.custom_name || relationNames[edge.relation_type] || edge.relation_type
const uniqueEdges = computed(() => {
  const seen = new Set<string>()
  const ids = new Set((props.graph?.nodes ?? []).map((n) => n.id))
  return (props.graph?.edges ?? []).filter((edge) => {
    if (!ids.has(edge.from_person_id) || !ids.has(edge.to_person_id)) return false
    const key = pairKey(edge.from_person_id, edge.to_person_id)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
})
const layout = computed(() => layoutFamilyGraph(props.graph?.nodes ?? [], props.graph?.edges ?? []))
const drawnEdges = computed<DrawnEdge[]>(() => {
  const map = layout.value.drawnEdgeByPair
  const out: DrawnEdge[] = []
  for (const edge of uniqueEdges.value) {
    const drawn = map.get(pairKey(edge.from_person_id, edge.to_person_id))
    if (!drawn) continue
    out.push(drawn)
  }
  return out
})

// ---- 模式切换：关系图 / 关系计算 ----
const viewMode = ref<'graph' | 'calc'>('graph')
const calcA = ref('')
const calcB = ref('')
const calcResult = ref<{ aToB: string; bToA: string } | null>(null)
function compute() {
  calcResult.value = calcA.value && calcB.value && props.graph ? computeKinship(calcA.value, calcB.value, props.graph.nodes, props.graph.edges) : null
}
function resetCalc() { calcA.value = ''; calcB.value = ''; calcResult.value = null }

// ---- 节点关系面板：点击节点列出全部关系，可直接解除；多关系可选 ----
const nodePanel = ref<{ personId: string; x: number; y: number } | null>(null)
const nodeRelations = computed(() => {
  const id = nodePanel.value?.personId
  if (!id) return []
  return drawnEdges.value.filter((e) => e.fromId === id || e.toId === id)
})
function openNodePanel(personId: string, event: any) {
  const stage = stageRef.value
  if (!stage) return
  const rect = stage.getBoundingClientRect()
  const client = event?.client ?? { x: rect.width / 2, y: rect.height / 2 }
  const x = Math.min(Math.max(client.x - rect.x + 18, 8), Math.max(8, rect.width - 316))
  const y = Math.min(Math.max(client.y - rect.y + 16, 8), Math.max(8, rect.height - 300))
  nodePanel.value = { personId, x, y }
}

// ---- 高亮：本地即时高亮，点击节点不再请求接口整图重载 ----
const highlightId = ref('')
const effectiveHighlight = computed(() => highlightId.value || props.graph?.center || '')
function applyHighlight(g: Graph | null = graphRef.value) {
  if (!g || !props.graph) return
  const hl = effectiveHighlight.value
  const stateMap: Record<string, string[]> = {}
  props.graph.nodes.forEach((n) => { stateMap[n.id] = [] })
  drawnEdges.value.forEach((e) => { stateMap[e.edge.id] = [] })
  if (hl) {
    stateMap[hl] = ['highlight']
    drawnEdges.value.forEach((e) => { if (e.fromId === hl || e.toId === hl) stateMap[e.edge.id] = ['active'] })
  }
  g.setElementState(stateMap, false)
}
function onNodeClick(id: string, event?: any) {
  if (!id) return
  highlightId.value = highlightId.value === id ? '' : id
  applyHighlight()
  emit('sync', highlightId.value)
  if (viewMode.value === 'calc') {
    // 计算模式：点选两个人物（先 A 后 B），弹出关系计算结果；不显示关系面板
    if (!calcA.value || calcB.value) { calcA.value = id; calcB.value = '' }
    else if (id !== calcA.value) calcB.value = id
    compute()
    return
  }
  if (nodePanel.value?.personId === id) nodePanel.value = null
  else openNodePanel(id, event)
}

// ---- 渲染 G6 图 ----
let renderChain: Promise<void> = Promise.resolve()
function scheduleRender() {
  renderChain = renderChain.then(doRender).catch((err) => {
    console.error('关系图渲染失败', err)
    Message.error(`关系图渲染失败：${err?.message || err}`)
  })
}
async function doRender() {
  // 等待容器就绪（图数据可能先于 DOM 更新到达）
  let container = stageRef.value
  for (let i = 0; i < 20 && !(container && props.graph); i++) {
    await new Promise((resolve) => setTimeout(resolve, 50))
    container = stageRef.value
  }
  if (!container || !props.graph) return
  const points = layout.value.points
  const nodes = props.graph.nodes.map((p) => {
    const pt = points.get(p.id) ?? { x: 0, y: 0 }
    return {
      id: p.id,
      data: { name: p.name },
      style: {
        fill: '#ffffff',
        stroke: '#5f8573',
        strokeWidth: 2.5,
        labelFill: '#26473c',
        shadowColor: 'rgba(39, 76, 57, 0.14)',
        shadowBlur: 10,
        shadowOffsetY: 3,
        cursor: 'pointer' as const,
      },
    }
  })
  const edges = drawnEdges.value.map((e) => {
    return {
      id: e.edge.id,
      source: e.fromId,
      target: e.toId,
      data: { label: e.label, derived: e.derived, sameLevel: e.sameLevel },
      style: {
        stroke: e.derived ? '#c8d3cc' : '#9fb8a9',
        strokeWidth: 2,
        lineDash: e.derived ? [6, 4] : undefined,
        opacity: e.derived ? 0.85 : 1,
        labelText: e.label,
        labelFill: '#8f5a45',
        labelFontSize: 12,
        labelBackground: true,
        labelBackgroundFill: '#ffffff',
        labelBackgroundRadius: 4,
        cursor: 'pointer' as const,
      },
    }
  })
  if (!graphRef.value) {
    graphRef.value = new Graph({
      container,
      data: { nodes, edges },
      autoResize: true,
      padding: 24,
      node: {
        type: 'rect',
        style: {
          size: [150, 58],
          radius: 6,
          labelText: (d) => String(d.data?.name || ''),
          labelPlacement: 'center',
          labelFontSize: 17,
          labelFontWeight: 600,
          labelFontFamily: "'Noto Serif SC', serif",
        },
        state: {
          highlight: { fill: '#f6ead6', stroke: '#b06b52', strokeWidth: 2.5, labelFill: '#7a4a33' },
          hover: { stroke: '#5f8a71', strokeWidth: 2.5 },
        },
      },
      edge: {
        type: (datum) => (datum.data?.sameLevel ? 'line' : 'cubic'),
        style: {
          labelFontSize: 13,
        },
        state: {
          active: { stroke: '#b06b52', strokeWidth: 2.4, labelBackgroundFill: '#f6ead6', labelFill: '#8f5a45' },
        },
      },
      behaviors: ['drag-canvas', 'zoom-canvas', 'drag-element'],
    })
    graphRef.value.on('node:click', (event: any) => onNodeClick(String(event?.target?.id || ''), event))
    graphRef.value.on('node:pointerenter', (event: any) => {
      const id = String(event?.target?.id || '')
      if (!id) return
      const states = id === effectiveHighlight.value ? ['highlight', 'hover'] : ['hover']
      graphRef.value?.setElementState({ [id]: states }, false)
    })
    graphRef.value.on('node:pointerleave', (event: any) => {
      const id = String(event?.target?.id || '')
      if (!id) return
      const states = id === effectiveHighlight.value ? ['highlight'] : []
      graphRef.value?.setElementState({ [id]: states }, false)
    })
    graphRef.value.on('edge:click', (event: any) => {
      const id = String(event?.target?.id || '')
      if (id) openEdgeById(id)
    })
    graphRef.value.on('wheel', () => {
      zoomLabel.value = `${Math.round((graphRef.value?.getZoom() ?? 1) * 100)}%`
    })
  } else {
    await graphRef.value.setData({ nodes, edges })
  }
  // 辈分分层坐标通过自定义布局注入（标准布局管线，保证节点正常绘制与后续重绘）
  const positions: Record<string, [number, number]> = {}
  props.graph.nodes.forEach((p) => {
    const pt = points.get(p.id)
    if (!pt) return
    positions[p.id] = [pt.x, pt.y]
  })
  await graphRef.value.setLayout({ type: 'family-positions', positions })
  await graphRef.value.render()
  await graphRef.value.fitView()
  applyHighlight()
  syncZoom()
}
watch(() => props.graph, () => { highlightId.value = ''; nodePanel.value = null; scheduleRender() })
onMounted(scheduleRender)
onBeforeUnmount(() => { graphRef.value?.destroy(); graphRef.value = null })

function syncZoom() { zoomLabel.value = `${Math.round((graphRef.value?.getZoom() ?? 1) * 100)}%` }
async function changeZoom(delta: number) { const g = graphRef.value; if (!g) return; const z = Math.min(1.8, Math.max(0.65, Math.round((g.getZoom() + delta) * 100) / 100)); await g.zoomTo(z); syncZoom() }
async function resetCanvas() { const g = graphRef.value; if (!g) return; await g.fitView(); syncZoom() }
async function downloadImage() {
  const g = graphRef.value
  if (!g) { Message.error('关系图尚未就绪'); return }
  try {
    const url = await g.toDataURL({ mode: 'overall', type: 'image/png' })
    const a = document.createElement('a')
    a.href = url
    a.download = `家族关系图-${new Date().toISOString().slice(0, 10)}.png`
    document.body.appendChild(a)
    a.click()
    a.remove()
    Message.success('关系图已开始下载')
  } catch (e: any) { Message.error('导出图片失败：' + (e?.message || e)) }
}

// ---- 点击关系 → 编辑弹窗 ----
const editVisible = ref(false)
const editSaving = ref(false)
const editingEdge = ref<Relation | null>(null)
const editCurrent = ref('')
const editForm = ref({ fromPersonID: '', toPersonID: '', relationType: 'FATHER', customName: '', note: '' })
function openEdgeById(id: string) {
  const e = drawnEdges.value.find((d) => d.edge.id === id)
  if (e) openEditEdge(e)
}
function openEditEdge(e: DrawnEdge) {
  const edge = e.edge
  if (edge.id.startsWith('derived:')) { Message.info('推导出的祖辈关系由父子关系自动生成，请编辑对应的父子关系'); return }
  editingEdge.value = edge
  editCurrent.value = `${props.personName(e.fromId)} · ${e.label} · ${props.personName(e.toId)}`
  editForm.value = { fromPersonID: e.fromId, toPersonID: e.toId, relationType: e.type, customName: edge.custom_name || '', note: edge.note || '' }
  editVisible.value = true
}
async function saveEditRelation() {
  const edge = editingEdge.value
  if (!edge || editSaving.value) return
  if (!editForm.value.fromPersonID || !editForm.value.toPersonID) { Message.error('请选择两位人物'); return }
  if (editForm.value.fromPersonID === editForm.value.toPersonID) { Message.error('不能选择同一个人物'); return }
  if (editForm.value.relationType === 'CUSTOM' && !editForm.value.customName.trim()) { Message.error('自定义关系需要填写关系名称'); return }
  editSaving.value = true
  try {
    await personApi.updateRelation(edge.id, { from_person_id: editForm.value.fromPersonID, to_person_id: editForm.value.toPersonID, relation_type: editForm.value.relationType, custom_name: editForm.value.customName, note: editForm.value.note })
    editVisible.value = false
    emit('refresh')
    Message.success('关系已更新')
  } catch (e: any) { Message.error(e.message) } finally { editSaving.value = false }
}
async function remove(id: string) { try { editVisible.value = false; await personApi.removeRelation(id); emit('refresh'); Message.success('关系已删除') } catch (e: any) { Message.error(e.message) } }
</script>

<template>
  <div class="g6-head">
    <div class="g6-head-main"><h2>家族关系地图</h2><p class="g6-sub">{{ graph ? '家族关系按辈分分层 · 点击人物查看并解除关系 · 点击关系线可编辑' : '选择一位人物作为高亮人物' }}</p></div>
    <div class="action-row g6-head-actions"><a-select :model-value="selectedPersonId || undefined" allow-search allow-clear placeholder="输入姓名搜索人物" style="width:200px" @change="emit('change', $event ? String($event) : '')"><a-option v-for="p in persons" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select><a-button type="primary" :disabled="!selectedPerson" @click="emit('add')">添加关系</a-button></div>
  </div>
  <div class="g6-toolbar">
    <div class="g6-toolbar-left"><div class="g6-trees"><button type="button" class="tree-tab" :class="{ active: viewMode === 'graph' }" @click="viewMode = 'graph'; nodePanel = null">关系图模式</button><button type="button" class="tree-tab" :class="{ active: viewMode === 'calc' }" @click="viewMode = 'calc'; nodePanel = null">关系计算模式</button></div><div v-if="graph && personFamilies && personFamilies.length > 1" class="g6-trees"><span class="g6-trees-label">家族树：</span><button v-for="f in personFamilies" :key="f.id" type="button" class="tree-tab" :class="{ active: f.id === familyId }" @click="f.id !== familyId && emit('tree', f.id)">{{ f.name }}{{ f.home ? '（主）' : '' }}</button></div><span class="g6-zoom">画布缩放 {{ zoomLabel }}</span><span class="g6-legend"><i class="g6-legend-solid"></i>正式关系<i class="g6-legend-dash"></i>推导祖辈</span></div>
    <div class="action-row"><a-button :disabled="zoomLabel === '65%'" @click="changeZoom(-.1)" size="mini">−</a-button><a-button @click="resetCanvas" size="mini">重置视图</a-button><a-button @click="downloadImage" size="mini">下载图片</a-button><a-button :disabled="zoomLabel === '180%'" @click="changeZoom(.1)" size="mini">＋</a-button></div>
  </div>
  <div v-if="viewMode === 'calc' && graph" class="calc-bar">
    <div class="calc-pickers"><a-select v-model="calcA" allow-search placeholder="人物 A（可点选图中人物）" style="width:200px" @change="compute"><a-option v-for="p in graph.nodes" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select><a-select v-model="calcB" allow-search placeholder="人物 B（可点选图中人物）" style="width:200px" @change="compute"><a-option v-for="p in graph.nodes" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select><a-button size="small" @click="resetCalc">清空</a-button></div>
    <div v-if="calcResult && calcA && calcB" class="calc-result"><p><strong>{{ personName(calcA) }}</strong> 是 <strong>{{ personName(calcB) }}</strong> 的 <em>{{ calcResult.aToB }}</em></p><p><strong>{{ personName(calcB) }}</strong> 是 <strong>{{ personName(calcA) }}</strong> 的 <em>{{ calcResult.bToA }}</em></p></div>
    <p v-else class="calc-hint">在图中依次点选两位人物（或从下拉选择），自动推导两人关系；支持父辈、同辈、孙辈、叔伯、侄甥、姨亲、堂亲、表亲，上下 18 代。</p>
  </div>
  <div class="graph-stage"><div v-if="graph" ref="stageRef" class="g6-stage" role="img" aria-label="家族人物关系图"></div><div v-else class="empty graph-empty">从人物索引中选择一位人物开始查看关系。</div><div v-if="nodePanel" class="node-panel" :style="{ left: nodePanel.x + 'px', top: nodePanel.y + 'px' }"><div class="node-panel-head">{{ personName(nodePanel.personId) }} 的关系（{{ nodeRelations.length }}）<button type="button" class="node-panel-close" @click="nodePanel = null">✕</button></div><div class="node-panel-list"><div v-for="e in nodeRelations" :key="e.edge.id" class="node-rel-row"><span class="node-rel-text"><strong :class="{ me: e.fromId === nodePanel.personId || e.toId === nodePanel.personId }">{{ personName(e.fromId) }}</strong> · {{ e.label }} · <strong :class="{ me: e.toId === nodePanel.personId || e.fromId === nodePanel.personId }">{{ personName(e.toId) }}</strong><template v-if="e.derived"><span class="g6-rel-derived-tag">推导</span></template></span><template v-if="!e.derived"><a-popconfirm content="确定解除该关系（含反向关系）？" @ok="remove(e.edge.id)"><a-button size="mini" type="text" status="danger">解除</a-button></a-popconfirm></template></div><div v-if="!nodeRelations.length" class="empty" style="padding:12px">暂无关系</div></div><div class="node-panel-foot">点击关系线可编辑 · 点击其他人物切换 · 点“添加关系”新建</div></div></div>
  <a-modal :visible="editVisible" title="编辑关系" :footer="false" :width="440" @update:visible="editVisible = $event">
    <div class="relation-form-grid" style="display:grid;gap:12px;grid-template-columns:1fr">
      <div class="edit-field">人物 A<a-select v-model="editForm.fromPersonID" :blur-to-close="false" allow-search placeholder="输入姓名搜索人物"><a-option v-for="p in persons" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></div>
      <div class="edit-field">关系类型<a-select v-model="editForm.relationType" :blur-to-close="false" allow-search><a-option value="FATHER">父亲</a-option><a-option value="MOTHER">母亲</a-option><a-option value="SON">儿子</a-option><a-option value="DAUGHTER">女儿</a-option><a-option value="HUSBAND">丈夫</a-option><a-option value="WIFE">妻子</a-option><a-option value="BROTHER">兄弟</a-option><a-option value="SISTER">姐妹</a-option><a-option value="GRANDFATHER">祖父</a-option><a-option value="GRANDMOTHER">祖母</a-option><a-option value="GRANDCHILD">孙辈</a-option><a-option value="UNCLE">叔伯</a-option><a-option value="AUNT">姑姨</a-option><a-option value="NEPHEW_NIECE">侄辈</a-option><a-option value="CUSTOM">自定义关系</a-option></a-select></div>
      <div class="edit-field">人物 B<a-select v-model="editForm.toPersonID" :blur-to-close="false" allow-search placeholder="输入姓名搜索人物"><a-option v-for="p in persons.filter((item) => item.id !== editForm.fromPersonID)" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></div>
      <div v-if="editForm.relationType === 'CUSTOM'" class="edit-field">关系名称<a-input v-model="editForm.customName" placeholder="例如：外公、义父、挚友" /></div>
      <div class="edit-field">备注（可选）<a-input v-model="editForm.note" placeholder="补充说明这段关系" /></div>
      <p class="edit-hint">关系含义：A 的〔关系类型〕是 B，例如 A=张宏均、B=张圣祥、关系=父亲，表示“张宏均的父亲是张圣祥”。<template v-if="editCurrent">当前：{{ editCurrent }}</template></p>
      <div class="action-row"><a-popconfirm content="确定删除该关系及其反向关系？" @ok="remove(editingEdge?.id || '')"><a-button status="danger">删除关系</a-button></a-popconfirm><a-button type="primary" :loading="editSaving" @click="saveEditRelation">保存修改</a-button></div>
    </div>
  </a-modal>
</template>
