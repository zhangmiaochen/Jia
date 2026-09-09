// 家族关系图分层（辈分）布局
// 将人物按代际（辈分）分层：长辈在上、晚辈在下、夫妻（同一辈分）并排，
// 并通过重心对齐减少连线交叉。

export interface LayoutGraphNode {
  id: string
  name: string
  gender?: string
}

export interface LayoutEdge {
  id: string
  from_person_id: string
  to_person_id: string
  relation_type: string
  custom_name: string
  note: string
}

export interface DrawnEdge {
  edge: LayoutEdge
  fromId: string
  toId: string
  sameLevel: boolean
  derived: boolean
  /** 规范化显示称谓：始终按“长辈 → 晚辈”方向（儿子/女儿/孙子/孙女…），
   *  与数据库中实际保存的是正向还是反向行无关 */
  label: string
  /** 编辑表单使用的规范关系类型（与 label 一致） */
  type: string
}

export interface LayoutResult {
  points: Map<string, { x: number; y: number }>
  drawnEdgeByPair: Map<string, DrawnEdge>
  width: number
  height: number
  maxRank: number
}

export const GRAPH_NODE_W = 148
export const GRAPH_NODE_H = 56

const SLOT = GRAPH_NODE_W + 34
const ROW_GAP = 152
const PAD_X = 90
const PAD_Y = 74

// 称谓约定：边的起点 A、终点 B，称谓描述“B 相对 A 的亲属称谓”（与后端关系约定一致）。
// 例如 (A→B, FATHER) 表示“B 是 A 的父亲” → 长辈是 B（终点）。
const ELDER_TO: Record<string, number> = { FATHER: 1, MOTHER: 1, GRANDFATHER: 2, GRANDMOTHER: 2, UNCLE: 1, AUNT: 1 }
const ELDER_FROM: Record<string, number> = { SON: 1, DAUGHTER: 1, GRANDCHILD: 2, NEPHEW_NIECE: 1 }
const SAME_LEVEL = new Set(['WIFE', 'HUSBAND', 'BROTHER', 'SISTER'])
const CUSTOM_ANCESTOR: Record<string, number> = { 父亲: 1, 爸爸: 1, 母亲: 1, 妈妈: 1, 祖父: 2, 祖母: 2, 爷爷: 2, 奶奶: 2, 外公: 2, 外婆: 2, 姥爷: 2, 姥姥: 2, 曾祖父: 3, 曾祖母: 3, 太爷爷: 3, 太奶奶: 3 }
const CUSTOM_DESCENDANT: Record<string, number> = { 儿子: 1, 女儿: 1, 侄子: 1, 侄女: 1, 外甥: 1, 外甥女: 1, 孙子: 2, 孙女: 2, 外孙: 2, 外孙女: 2, 曾孙: 3, 曾孙女: 3, 孙辈: 2 }

interface Orientation {
  elderId?: string
  youngerId?: string
  diff: number
  sameLevel: boolean
  couple: boolean
  custom?: boolean
}

const DEFAULT_LABEL: Record<string, string> = { FATHER: '父亲', MOTHER: '母亲', SON: '儿子', DAUGHTER: '女儿', HUSBAND: '丈夫', WIFE: '妻子', BROTHER: '兄弟', SISTER: '姐妹', GRANDFATHER: '祖父', GRANDMOTHER: '祖母', GRANDCHILD: '孙辈', UNCLE: '叔伯', AUNT: '姑姨', NEPHEW_NIECE: '侄辈', MATERNAL_GRANDFATHER: '外祖父', MATERNAL_GRANDMOTHER: '外祖母' }

function orientEdge(e: LayoutEdge): Orientation {
  const t = e.relation_type
  if (ELDER_TO[t]) return { elderId: e.to_person_id, youngerId: e.from_person_id, diff: ELDER_TO[t], sameLevel: false, couple: false }
  if (ELDER_FROM[t]) return { elderId: e.from_person_id, youngerId: e.to_person_id, diff: ELDER_FROM[t], sameLevel: false, couple: false }
  if (SAME_LEVEL.has(t)) return { diff: 0, sameLevel: true, couple: t === 'WIFE' || t === 'HUSBAND' }
  // 自定义称谓：存入方向对同一条关系会写入两条相同标签的边，方向不可靠，
  // 不参与辈分计算，绘制时按实际辈分定方向（见 drawnEdges 构建）。
  const name = (e.custom_name || '').trim()
  const da = CUSTOM_ANCESTOR[name]
  if (da) return { elderId: e.to_person_id, youngerId: e.from_person_id, diff: da, sameLevel: false, couple: false, custom: true }
  const dd = CUSTOM_DESCENDANT[name]
  if (dd) return { elderId: e.from_person_id, youngerId: e.to_person_id, diff: dd, sameLevel: false, couple: false, custom: true }
  return { diff: 0, sameLevel: true, couple: false, custom: true }
}

// 孙辈以下代际称谓（第 2~9 代：孙/曾孙/玄孙/来孙/晜孙/仍孙/云孙/耳孙；10 代以上用 第N世孙）
const DESC_NAMES: [string, string][] = [
  ['孙子', '孙女'], ['曾孙', '曾孙女'], ['玄孙', '玄孙女'], ['来孙', '来孙女'],
  ['晜孙', '晜孙女'], ['仍孙', '仍孙女'], ['云孙', '云孙女'], ['耳孙', '耳孙女'],
]
const CN_NUM = ['十', '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八']
function derivedLabel(d: number, female: boolean, maternal: boolean) {
  if (d === 2) return female ? (maternal ? '外孙女' : '孙女') : maternal ? '外孙' : '孙子'
  if (d <= 9) return (maternal ? '外' : '') + DESC_NAMES[d - 2][female ? 1 : 0]
  return `第${CN_NUM[d - 10] || d}世${maternal ? '外' : ''}孙${female ? '女' : ''}`
}
// 推导边类型 → 代际与外/内系：G2（祖）/ M2（外祖）/ G3+ / M3+（曾祖以上）
function parseDerivedType(t: string) {
  const m = /^([GM])(\d{1,2})$/.exec(t)
  if (m) return { d: Number(m[2]), maternal: m[1] === 'M' }
  if (t === 'MATERNAL_GRANDFATHER' || t === 'MATERNAL_GRANDMOTHER') return { d: 2, maternal: true }
  return { d: 2, maternal: false }
}

function pairKey(fromId: string, toId: string) {
  return [fromId, toId].sort().join('\u0001')
}
export { pairKey }

// 规范称谓：无论关系在库中保存的是“正向”还是“反向”行，
// 展示与编辑统一使用“长辈 → 晚辈”方向（儿子/女儿/孙子/孙女/侄子/侄女…）。
function canonicalOf(x: { edge: LayoutEdge; o: Orientation }, genderMap: Map<string, string>) {
  const t = x.edge.relation_type
  if (x.o.custom) return { label: (x.edge.custom_name || '').trim() || '自定义', type: t }
  if (x.o.sameLevel) return { label: DEFAULT_LABEL[t] || t, type: t }
  const younger = x.o.youngerId ?? ''
  const g = genderMap.get(younger)
  if (['FATHER', 'MOTHER', 'SON', 'DAUGHTER'].includes(t)) {
    return g === 'female' ? { label: '女儿', type: 'DAUGHTER' } : { label: '儿子', type: 'SON' }
  }
  if (['GRANDFATHER', 'GRANDMOTHER', 'GRANDCHILD'].includes(t)) {
    return g === 'female' ? { label: '孙女', type: 'GRANDCHILD' } : { label: '孙子', type: 'GRANDCHILD' }
  }
  if (['MATERNAL_GRANDFATHER', 'MATERNAL_GRANDMOTHER'].includes(t)) {
    return g === 'female' ? { label: '外孙女', type: 'GRANDCHILD' } : { label: '外孙', type: 'GRANDCHILD' }
  }
  if (['UNCLE', 'AUNT', 'NEPHEW_NIECE'].includes(t)) {
    return g === 'female' ? { label: '侄女', type: 'NEPHEW_NIECE' } : { label: '侄子', type: 'NEPHEW_NIECE' }
  }
  return { label: DEFAULT_LABEL[t] || t, type: t }
}

interface Unit {
  members: string[]
  bary: number
}

export function layoutFamilyGraph(nodes: LayoutGraphNode[], rawEdges: LayoutEdge[]): LayoutResult {
  const points = new Map<string, { x: number; y: number }>()
  if (!nodes.length) return { points, drawnEdgeByPair: new Map(), width: 1100, height: 650, maxRank: 0 }

  const ids = new Set(nodes.map((n) => n.id))
  // 去重：同一对人物保留一条关系（先出现的为准）
  const seen = new Set<string>()
  const unique: LayoutEdge[] = []
  for (const e of rawEdges) {
    if (!ids.has(e.from_person_id) || !ids.has(e.to_person_id)) continue
    const key = pairKey(e.from_person_id, e.to_person_id)
    if (seen.has(key)) continue
    seen.add(key)
    unique.push(e)
  }
  const oriented = unique.map((edge) => ({ edge, o: orientEdge(edge), derived: edge.id.startsWith('derived:') }))

  // ---- 辈分（rank）计算：长辈约束 + 夫妻同层 ----
  const rankOf = new Map<string, number>()
  nodes.forEach((n) => rankOf.set(n.id, 0))
  const constraints = oriented.filter((x) => !x.derived && !x.o.custom && x.o.diff > 0)
  const couples = new Set<string>()
  oriented.forEach((x) => {
    if (x.o.couple) couples.add(pairKey(x.edge.from_person_id, x.edge.to_person_id))
  })
  for (let iter = 0; iter < 8; iter++) {
    let changed = false
    for (const c of constraints) {
      const u = rankOf.get(c.o.elderId!)!
      const v = rankOf.get(c.o.youngerId!)!
      const target = u + c.o.diff
      if (v < target) { rankOf.set(c.o.youngerId!, target); changed = true }
    }
    for (const key of couples) {
      const [a, b] = key.split('\u0001')
      if (!rankOf.has(a) || !rankOf.has(b)) continue
      const m = Math.max(rankOf.get(a)!, rankOf.get(b)!)
      if (rankOf.get(a)! !== m || rankOf.get(b)! !== m) { rankOf.set(a, m); rankOf.set(b, m); changed = true }
    }
    if (!changed) break
  }
  const minRank = Math.min(...Array.from(rankOf.values()))
  if (minRank !== 0) rankOf.forEach((v, k) => rankOf.set(k, v - minRank))

  // 孤立人物（无任何关系）单独放最底下一行
  const linked = new Set<string>()
  oriented.forEach((x) => { linked.add(x.edge.from_person_id); linked.add(x.edge.to_person_id) })
  let maxRank = Math.max(0, ...Array.from(rankOf.values()))
  nodes.forEach((n) => { if (!linked.has(n.id)) rankOf.set(n.id, maxRank + 1) })
  maxRank = Math.max(maxRank, ...Array.from(rankOf.values()))

  // ---- 夫妻成组（单元），同层相邻；夫左妻右 ----
  const partner = new Map<string, string>()
  oriented.forEach((x) => {
    if (!x.o.couple) return
    const a = x.edge.from_person_id
    const b = x.edge.to_person_id
    if (!partner.has(a) && !partner.has(b)) { partner.set(a, b); partner.set(b, a) }
  })
  const gender = new Map<string, string>()
  nodes.forEach((n) => { if (n.gender) gender.set(n.id, n.gender) })

  const levels: string[][] = []
  for (let r = 0; r <= maxRank; r++) levels.push([])
  nodes.forEach((n) => levels[rankOf.get(n.id)!].push(n.id))

  // 长辈/晚辈邻接（用于重心对齐）
  const eldersOf = new Map<string, string[]>()
  const childrenOf = new Map<string, string[]>()
  const addAdj = (map: Map<string, string[]>, k: string, v: string) => {
    const list = map.get(k)
    if (list) list.push(v)
    else map.set(k, [v])
  }
  oriented.forEach((x) => {
    if (x.derived || x.o.diff <= 0) return
    addAdj(eldersOf, x.o.youngerId!, x.o.elderId!)
    addAdj(childrenOf, x.o.elderId!, x.o.youngerId!)
  })

  const x = new Map<string, number>()

  const unitList = (level: string[]): Unit[] => {
    const used = new Set<string>()
    const units: Unit[] = []
    for (const id of level) {
      if (used.has(id)) continue
      const p = partner.get(id)
      if (p && level.includes(p) && !used.has(p)) {
        used.add(id); used.add(p)
        const a = gender.get(id) === 'male' ? id : p
        const b = gender.get(id) === 'male' ? p : id
        units.push({ members: [a, b], bary: 0 })
      } else {
        used.add(id)
        units.push({ members: [id], bary: 0 })
      }
    }
    return units
  }

  const neighborAvg = (unit: Unit, neighborMap: Map<string, string[]>) => {
    let sum = 0
    let n = 0
    for (const m of unit.members) {
      for (const e of neighborMap.get(m) || []) {
        if (x.has(e)) { sum += x.get(e)!; n++ }
        else { sum += PAD_X; n++ }
      }
    }
    return n ? sum / n : NaN
  }
  const orderLevel = (r: number, neighborMap: Map<string, string[]>) => {
    const units = unitList(levels[r])
    for (const u of units) {
      const v = neighborAvg(u, neighborMap)
      u.bary = Number.isFinite(v) ? v : r === 0 ? 0 : 1e9
    }
    units.sort((a, b) => (a.bary - b.bary) || a.members[0].localeCompare(b.members[0], 'zh-Hans-CN'))
    return units
  }
  const assignX = (units: Unit[]) => {
    const slots = units.reduce((s, u) => s + u.members.length, 0)
    const width = slots * SLOT
    let cursor = (Math.max(1100, width + PAD_X * 2) - width) / 2
    units.forEach((u) => {
      u.members.forEach((m, i) => { x.set(m, cursor + i * SLOT + GRAPH_NODE_W / 2) })
      cursor += u.members.length * SLOT
    })
  }

  // 自上而下：按长辈重心排序一次；自下而上：按晚辈重心重排一次（子女拉齐父母）
  for (let r = 0; r <= maxRank; r++) assignX(orderLevel(r, eldersOf))
  for (let r = maxRank; r >= 0; r--) levels[r] = orderLevel(r, childrenOf).map((u) => u.members).flat()
  for (let r = 0; r <= maxRank; r++) assignX(orderLevel(r, childrenOf))

  // ---- 输出坐标 ----
  nodes.forEach((n) => {
    points.set(n.id, { x: x.get(n.id) ?? 0, y: PAD_Y + rankOf.get(n.id)! * ROW_GAP + GRAPH_NODE_H / 2 })
  })

  // ---- 可见边（方向修正 + 推导边标记 + 规范称谓） ----
  const drawnEdgeByPair = new Map<string, DrawnEdge>()
  oriented.forEach((x) => {
    const rA = rankOf.get(x.edge.from_person_id) ?? 0
    const rB = rankOf.get(x.edge.to_person_id) ?? 0
    let fromId: string
    let toId: string
    if (x.derived) {
      // 推导边（祖辈→后辈）：方向与称谓按辈分与晚辈性别独立确定，不依赖后端行方向；
      // 支持深代推导（G3+ 曾祖以上 / M3+ 外系），称谓按代际与内外系生成。
      fromId = rA <= rB ? x.edge.from_person_id : x.edge.to_person_id
      toId = rA <= rB ? x.edge.to_person_id : x.edge.from_person_id
      const g = gender.get(toId)
      const { d, maternal } = parseDerivedType(x.edge.relation_type)
      drawnEdgeByPair.set(pairKey(x.edge.from_person_id, x.edge.to_person_id), {
        edge: x.edge,
        fromId,
        toId,
        sameLevel: false,
        derived: true,
        label: derivedLabel(d, g === 'female', maternal),
        type: 'GRANDCHILD',
      })
      return
    }
    if (x.o.sameLevel) {
      fromId = x.edge.from_person_id
      toId = x.edge.to_person_id
    } else if (x.o.custom) {
      // 自定义称谓方向不确定：辈分相同则水平绘制；否则让线从长辈画向晚辈
      fromId = rA <= rB ? x.edge.from_person_id : x.edge.to_person_id
      toId = rA <= rB ? x.edge.to_person_id : x.edge.from_person_id
    } else {
      fromId = x.o.elderId!
      toId = x.o.youngerId!
    }
    const canonical = canonicalOf(x, gender)
    drawnEdgeByPair.set(pairKey(x.edge.from_person_id, x.edge.to_person_id), {
      edge: x.edge,
      fromId,
      toId,
      sameLevel: x.o.sameLevel || (x.o.custom === true && rA === rB),
      derived: false,
      label: canonical.label,
      type: canonical.type,
    })
  })

  const maxSlots = Math.max(...levels.map((l) => l.length))
  const contentW = maxSlots * SLOT
  const contentH = PAD_Y * 2 + maxRank * ROW_GAP + GRAPH_NODE_H
  return {
    points,
    drawnEdgeByPair,
    width: Math.max(1100, contentW + PAD_X * 2),
    height: Math.max(650, contentH),
    maxRank,
  }
}
