// 亲属关系计算（正规化实现）
// 模型：血亲树（父母→子女，仅一层边）+ 夫妻联合（同代、共享后代、各自保留原生父母）。
// 称谓规则（相对"本人"）：
//   直系：父/母、祖父/母、曾祖父/母…（女儿线路 ⇒ 外系"外"前缀）
//   旁系同代：兄弟/姐妹、堂(全男线)/表(其余)…兄弟/姐妹、第N代堂/表亲
//   旁系交叉：伯叔/舅、姑母/姨母（+祖链）；侄子/侄女、外甥/外甥女（+孙链）
//   婚姻配偶：儿媳/女婿、孙媳/孙女婿…… 公婆/岳父母、伯母/婶娘、姑父/姨父、舅母……
//   同辈姻亲：嫂子/弟媳、姐夫/妹夫、大/小姑子、大/小舅子、妯娌、连襟；亲家公/亲家母

export interface KinNode {
  id: string
  gender?: string
}

export interface KinEdge {
  from_person_id: string
  to_person_id: string
  relation_type: string
  custom_name: string
}

const CN = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十', '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八']
const CN10 = ['十', '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八']
const DESC_CHAIN = ['', '子', '孙', '曾孙', '玄孙', '来孙', '晜孙', '仍孙', '云孙', '耳孙']
const ANC_CHAIN = ['', '父', '祖', '曾祖', '高祖', '天祖', '烈祖', '太祖', '远祖', '鼻祖']

const CUSTOM_ANCESTOR: Record<string, number> = { 父亲: 1, 爸爸: 1, 母亲: 1, 妈妈: 1, 祖父: 2, 祖母: 2, 爷爷: 2, 奶奶: 2, 外公: 2, 外婆: 2, 姥爷: 2, 姥姥: 2 }
const CUSTOM_DESCENDANT: Record<string, number> = { 儿子: 1, 女儿: 1, 孙子: 2, 孙女: 2, 外孙: 2, 外孙女: 2, 曾孙: 3, 曾孙女: 3 }
// 自定义姻亲称谓 → 代际差（正数：B 比 A 低 N 代，即"A 的〔称谓〕是 B"）
const CUSTOM_INLAW_DOWN: Record<string, number> = {
  儿媳: 1, 媳妇: 1, 儿媳妇: 1, 大儿媳: 1, 二儿媳: 1, 三儿媳: 1, 四儿媳: 1, 女婿: 1,
  孙媳: 2, 孙媳妇: 2, 孙女婿: 2, 曾孙媳: 3, 曾孙女婿: 3,
}
const CUSTOM_INLAW_UP: Record<string, number> = { 公公: 1, 婆婆: 1, 岳父: 1, 岳母: 1, 公婆: 1 }

function descTerm(depth: number, maternal: boolean, female: boolean): string {
  if (depth < 1) return ''
  if (depth === 1) return female ? '女儿' : '儿子'
  let w = ''
  if (depth <= 9) w = DESC_CHAIN[depth]
  else w = `第${CN10[Math.min(depth - 10, CN10.length - 1)]}世孙`
  if (maternal) w = '外' + w
  return female ? w + '女' : w
}

function ancTerm(depth: number, maternal: boolean, femaleAncestor: boolean): string {
  if (depth < 1) return ''
  if (depth === 1) return femaleAncestor ? '母亲' : '父亲'
  let w = ''
  if (depth <= 9) w = ANC_CHAIN[depth]
  else w = `第${CN10[Math.min(depth - 10, CN10.length - 1)]}世祖`
  if (maternal) w = '外' + w
  return w + (femaleAncestor ? '母' : '父')
}

/** 某人的配偶在称谓上的"对位上辈"：谁与谁比决定 父→公公/岳父（依中间那人的性别） */
function spouseOfRelView(role: string, personFemale: boolean): string {
  if (role === '父亲') return personFemale ? '公公' : '岳父'
  if (role === '母亲') return personFemale ? '婆婆' : '岳母'
  // 表/堂兄弟（姐妹）在"对方配偶"视角下同辈沿用（表兄弟/表姐妹——代际不变）
  const cousinB = /^(堂|姨表|舅表|姑表|表)?(兄弟|姐妹)$/.exec(role)
  if (cousinB) return role
  return role
}

/** 某人（其角色为 role）的配偶在称谓上的称谓：X 是 B 的〔role〕→ X 的配偶是 B 的〔?〕 */
function spouseView(role: string, spouseFemale: boolean): string {
  const DESC: Record<string, string> = {
    儿子: '儿媳', 女儿: '女婿', 孙子: '孙媳', 孙女: '孙女婿', 曾孙: '曾孙媳', 曾孙女: '曾孙女婿',
    玄孙: '玄孙媳', 玄孙女: '玄孙女婿', 来孙: '来孙媳', 来孙女: '来孙女婿',
    晜孙: '晜孙媳', 晜孙女: '晜孙女婿', 仍孙: '仍孙媳', 仍孙女: '仍孙女婿',
    云孙: '云孙媳', 云孙女: '云孙女婿', 耳孙: '耳孙媳', 耳孙女: '耳孙女婿',
    侄子: '侄媳妇', 侄女: '侄女婿', 侄孙: '侄孙媳妇', 侄孙女: '侄孙女婿',
    外甥: '外甥媳妇', 外甥女: '外甥女婿', 甥孙: '甥孙媳妇', 甥孙女: '甥孙女婿',
    兄弟: '嫂子/弟媳', 姐妹: '姐夫/妹夫', 表兄弟: '表嫂子/表弟媳', 表姐妹: '表姐夫/表妹夫', 堂兄弟: '堂嫂子/堂弟媳', 堂姐妹: '堂姐夫/堂妹夫',
  }
  if (DESC[role]) return DESC[role]
  // 深代子孙（第N世孙）的配偶：第N世孙媳/婿
  const deepDesc = /^第.+世孙$/.exec(role)
  if (deepDesc) return role + (spouseFemale ? '媳' : '婿')
  // 表/堂兄弟（姐妹）的配偶：表嫂子/表弟媳、姨表嫂子…
  const cousin = /^(堂|姨表|舅表|姑表|表)?(兄弟|姐妹)$/.exec(role)
  if (cousin) {
    const k = cousin[1] || ''
    const sib = cousin[2]
    if (k === '') return sib === '兄弟' ? '嫂子/弟媳' : '姐夫/妹夫'
    if (k === '堂') return sib === '兄弟' ? '堂嫂子/堂弟媳' : '堂姐夫/堂妹夫'
    if (k === '表') return sib === '兄弟' ? '表嫂子/表弟媳' : '表姐夫/表妹夫'
    return sib === '兄弟' ? `${k}嫂子/${k}弟媳` : `${k}姐夫/${k}妹夫`
  }
  // 长辈旁系：换性（伯叔的配偶=伯母/婶娘，姑的配偶=姑父……）
  if (role === '伯叔' || role === '叔') return spouseFemale ? '伯母/婶娘' : '伯叔父'
  if (role.endsWith('伯')) return spouseFemale ? role + '母' : role + '父'
  if (role.endsWith('祖父')) return role.slice(0, -1) + '母'
  if (role.endsWith('祖母')) return role.slice(0, -1) + '父'
  if (role === '舅') return spouseFemale ? '舅母' : '舅父'
  if (role === '姑') return spouseFemale ? '姑母' : '姑父'
  if (role === '姨') return spouseFemale ? '姨母' : '姨父'
  if (role.endsWith('父')) return role.slice(0, -1) + '母'
  if (role.endsWith('母')) return role.slice(0, -1) + '父'
  return role
}

type KinResult = { aToB: string; bToA: string; score: number } | null

/** 内部原始计算（含分数，分数越小越优）；导出层做双方向对称择优 */
function computeKinshipRaw(aId: string, bId: string, nodes: KinNode[], edges: KinEdge[], depth = 0, seen?: Set<string>): KinResult {
  if (!aId || !bId) return null
  const seenNext = new Set(seen || [])
  const seenKey = aId + '\u0002' + bId
  if (seenNext.has(seenKey)) return null
  seenNext.add(seenKey)
  if (aId === bId) return { aToB: '本人', bToA: '本人', score: 0 }
  const genderOf = new Map<string, string>()
  nodes.forEach((n) => genderOf.set(n.id, n.gender || ''))

  // 1) 数据构建：血亲子、夫妻、自定义姻亲（按称谓语义 + 晚辈性别定方向）
  const parentsOf = new Map<string, string[]>()
  const addParent = (child: string, parent: string) => {
    const list = parentsOf.get(child)
    if (list) { if (!list.includes(parent)) list.push(parent) } else parentsOf.set(child, [parent])
  }
  const inLawDown = new Map<string, { id: string; dist: number; kind: string }[]>()
  const addInLaw = (elder: string, younger: string, dist: number, kind: string) => {
    const list = inLawDown.get(elder)
    if (list) { if (!list.some((x) => x.id === younger)) list.push({ id: younger, dist, kind }) } else inLawDown.set(elder, [{ id: younger, dist, kind }])
  }
  const spousesOf = new Map<string, string[]>()
  const addSpouse = (a: string, b: string) => {
    const list = spousesOf.get(a)
    if (list) { if (!list.includes(b)) list.push(b) } else spousesOf.set(a, [b])
  }
  let couple = false
  for (const e of edges) {
    const t = e.relation_type
    // 约定：A→B 的称谓描述 B。FATHER/MOTHER → B 是 A 的父母；SON/DAUGHTER → B 是 A 的子女
    if (t === 'FATHER' || t === 'MOTHER') addParent(e.from_person_id, e.to_person_id)
    else if (t === 'SON' || t === 'DAUGHTER') addParent(e.to_person_id, e.from_person_id)
    else if (t === 'HUSBAND' || t === 'WIFE') {
      addSpouse(e.from_person_id, e.to_person_id)
      addSpouse(e.to_person_id, e.from_person_id)
      if ((e.from_person_id === aId && e.to_person_id === bId) || (e.from_person_id === bId && e.to_person_id === aId)) couple = true
    } else if (t === 'CUSTOM') {
      const name = (e.custom_name || '').trim()
      const da = CUSTOM_ANCESTOR[name]
      const dd = CUSTOM_DESCENDANT[name]
      if (da === 1) addParent(e.from_person_id, e.to_person_id)
      else if (dd === 1) addParent(e.to_person_id, e.from_person_id)
      else {
        const down = CUSTOM_INLAW_DOWN[name]
        const up = CUSTOM_INLAW_UP[name]
        let kind = ''
        let dist = 0
        if (down) { dist = down; kind = name.endsWith('婿') ? 'xu' : 'xi' }
        else if (up) { dist = up; kind = name.includes('婿') || name.includes('岳') ? 'xu' : 'xi' }
        if (kind && dist > 0) {
          // 按"晚辈性别与称谓匹配"选定方向（女婿→男、儿媳→女；长辈称谓反向）
          const want = kind === 'xu' ? 'male' : 'female'
          if (down && genderOf.get(e.to_person_id) === want) addInLaw(e.from_person_id, e.to_person_id, dist, kind)
          else if (down && genderOf.get(e.from_person_id) === want) addInLaw(e.to_person_id, e.from_person_id, dist, kind)
          else if (up && genderOf.get(e.from_person_id) === want) addInLaw(e.to_person_id, e.from_person_id, dist, kind)
          else if (up && genderOf.get(e.to_person_id) === want) addInLaw(e.from_person_id, e.to_person_id, dist, kind)
        }
        // 未识别的自定义（同事/亲友等）：不作结构假设
      }
    }
  }
  if (couple) {
    const aToB = genderOf.get(aId) === 'female' ? '妻子' : '丈夫'
    const bToA = genderOf.get(aId) === 'female' ? '丈夫' : '妻子'
    return { aToB, bToA, score: 0 }
  }

  // 2) 兄弟/姐妹：共享至少一位血亲父母
  const parentsA = parentsOf.get(aId) || []
  const parentsB = parentsOf.get(bId) || []
  if (parentsA.some((p) => parentsB.includes(p))) {
    const bToA = genderOf.get(bId) === 'female' ? '姐妹' : '兄弟'
    const aToB = genderOf.get(aId) === 'female' ? '姐妹' : '兄弟'
    return { aToB, bToA, score: 0 }
  }

  const childrenOf = new Map<string, string[]>()
  parentsOf.forEach((ps, child) => ps.forEach((p) => {
    const l = childrenOf.get(p)
    if (l) l.push(child)
    else childrenOf.set(p, [child])
  }))
  const isBloodDescendant = (anc: string, desc: string): boolean => {
    const stack: string[] = [anc]
    const visited = new Set<string>([anc])
    while (stack.length) {
      const cur = stack.pop()!
      if (cur === desc) return true
      for (const child of childrenOf.get(cur) || []) {
        if (!visited.has(child)) { visited.add(child); stack.push(child) }
      }
    }
    return false
  }
  const hasAllMaleChildPath = (anc: string, desc: string): boolean => {
    const stack: [string, boolean][] = [[anc, true]]
    const visited = new Set<string>([anc])
    while (stack.length) {
      const [cur, ok] = stack.pop()!
      if (cur === desc) { if (ok) return true; continue }
      for (const child of childrenOf.get(cur) || []) {
        if (visited.has(child)) continue
        visited.add(child)
        stack.push([child, child === desc ? ok : ok && genderOf.get(child) !== 'female'])
      }
    }
    return false
  }
  // 自定义姻亲直链（带代际与系别）
  const inLawDirect = (anc: string, desc: string): { depth: number; kind: string } | null => {
    const stack: [string, number, string][] = [[anc, 0, '']]
    const visited = new Set<string>([anc])
    let bestDepth: number | null = null
    let bestKind = ''
    while (stack.length) {
      const [cur, depth, kind] = stack.pop()!
      if (cur === desc) {
        if (depth >= 1 && depth <= 3 && (bestDepth === null || depth < bestDepth)) { bestDepth = depth; bestKind = kind }
        continue
      }
      for (const child of childrenOf.get(cur) || []) {
        if (visited.has(child)) continue
        visited.add(child)
        stack.push([child, depth + 1, kind])
      }
      for (const il of inLawDown.get(cur) || []) {
        if (visited.has(il.id)) continue
        visited.add(il.id)
        stack.push([il.id, depth + il.dist, il.kind])
      }
    }
    return bestDepth !== null ? { depth: bestDepth, kind: bestKind } : null
  }

  // 3) 同辈姻亲（无血缘）：配偶是对方血亲兄弟/姐妹、或双方配偶互为血亲兄妹
  const aFemale0 = genderOf.get(aId) === 'female'
  const bFemale0 = genderOf.get(bId) === 'female'
  const bloodSibling = (p: string, q: string) => (parentsOf.get(p) || []).some((x) => (parentsOf.get(q) || []).includes(x))
  const sameGenInLaw = ((): { aToB: string; bToA: string } | null => {
    const xMale = (x: string) => genderOf.get(x) === 'male'
    for (const x of spousesOf.get(bId) || []) {
      if (bloodSibling(x, aId)) {
        // X 是 A 的血亲兄弟/姐妹，B 是 X 的配偶
        return xMale(x)
          ? { aToB: aFemale0 ? '大姑子/小姑子' : '大伯子/小叔子', bToA: '嫂子/弟媳' }
          : { aToB: aFemale0 ? '小姨子/大姨子' : '小舅子/大舅子', bToA: '姐夫/妹夫' }
      }
    }
    for (const x of spousesOf.get(aId) || []) {
      if (bloodSibling(x, bId)) {
        // X 是 B 的血亲兄弟/姐妹，A 是 X 的配偶
        return xMale(x)
          ? { aToB: '嫂子/弟媳', bToA: bFemale0 ? '大姑子/小姑子' : '大伯子/小叔子' }
          : { aToB: '姐夫/妹夫', bToA: bFemale0 ? '小姨子/大姨子' : '小舅子/大舅子' }
      }
    }
    for (const x of spousesOf.get(aId) || []) {
      for (const y of spousesOf.get(bId) || []) {
        if (bloodSibling(x, y)) {
          if (aFemale0 && bFemale0) return { aToB: '妯娌', bToA: '妯娌' }
          if (!aFemale0 && !bFemale0) return { aToB: '连襟', bToA: '连襟' }
          // 一男一女：配偶互为血亲姐(妹)弟 → 姐夫/妹夫 ↔ 妻嫂/妻弟媳
          if (!aFemale0 && bFemale0) return { aToB: '姐夫/妹夫', bToA: '妻嫂/妻弟媳' }
          return { aToB: '妻嫂/妻弟媳', bToA: '姐夫/妹夫' }
        }
      }
    }
    return null
  })()
  if (sameGenInLaw) return { ...sameGenInLaw, score: 1 }
  // 亲家：一方血亲后代之配偶的父母是另一方
  const affinalParent = ((startId: string, otherId: string): { aToB: string; bToA: string } | null => {
    const stack: string[] = [startId]
    const visited = new Set<string>([startId])
    while (stack.length) {
      const cur = stack.pop()!
      for (const child of childrenOf.get(cur) || []) {
        if (visited.has(child)) continue
        visited.add(child)
        for (const s of spousesOf.get(child) || []) {
          if ((parentsOf.get(s) || []).includes(otherId)) {
            return { aToB: aFemale0 ? '亲家母' : '亲家公', bToA: bFemale0 ? '亲家母' : '亲家公' }
          }
        }
        stack.push(child)
      }
    }
    return null
  })
  const aff1 = affinalParent(aId, bId)
  if (aff1) return { ...aff1, score: 2 }
  const aff2 = affinalParent(bId, aId)
  if (aff2) return { ...aff2, score: 3 }

  // 4) 最近共同祖先（仅纯血缘）
  const MAXD = 18
  const levelsA = ancestorLevels(aId, parentsOf, MAXD)
  const levelsB = ancestorLevels(bId, parentsOf, MAXD)
  const best = levelsA && levelsB ? findLCA(levelsA, levelsB) : null
  const m = best ? best.la : 0
  const n = best ? best.lb : 0

  // 配偶路线（无血缘关联时）：一方的配偶与另一方有纯血缘关系 → 以配偶侧称谓换算
  // （seen 集合防止递归环；深层同样适用，保证多级姻亲链可达）
  if (!best) {
    const tryView = (r: KinResult, spouseOfA: boolean): KinResult => {
      if (!r) return null
      const view = spouseOfA ? spouseView(r.aToB, aFemale0) : spouseOfRelView(r.aToB, bFemale0)
      const viewBack = spouseOfA ? spouseOfRelView(r.bToA, aFemale0) : spouseView(r.bToA, bFemale0)
      if (view === r.aToB && viewBack === r.bToA) return null
      return { aToB: view, bToA: viewBack, score: r.score }
    }
    for (const y of spousesOf.get(bId) || []) {
      if (y === aId) continue
      const r = computeKinshipRaw(aId, y, nodes, edges, depth + 1, seenNext)
      const v = tryView(r, false)
      if (v) return v
    }
    for (const x of spousesOf.get(aId) || []) {
      if (x === bId) continue
      const r = computeKinshipRaw(x, bId, nodes, edges, depth + 1, seenNext)
      const v = tryView(r, true)
      if (v) return v
    }
  }
  if (!best) {
    // 自定义姻亲直链兜底（如"大儿媳"等称谓边）
    const il1 = inLawDirect(aId, bId)
    if (il1) return { aToB: inLawAncTerm(il1.depth, il1.kind, genderOf.get(aId) === 'female'), bToA: inLawDescTerm(il1.depth, il1.kind, genderOf.get(bId) === 'female'), score: 40 + il1.depth }
    const il2 = inLawDirect(bId, aId)
    if (il2) return { aToB: inLawDescTerm(il2.depth, il2.kind, genderOf.get(aId) === 'female'), bToA: inLawAncTerm(il2.depth, il2.kind, genderOf.get(bId) === 'female'), score: 40 + il2.depth }
    return null
  }

  // 5) 直系：A 是 B 的长辈 / B 是 A 的长辈（外系 = 不存在"中间全男性"的血缘路径）
  if (m === 0) {
    if (hasAllMaleChildPath(aId, bId)) return { aToB: ancTerm(n, false, genderOf.get(aId) === 'female'), bToA: descTerm(n, false, genderOf.get(bId) === 'female'), score: n }
    if (isBloodDescendant(aId, bId)) return { aToB: ancTerm(n, true, genderOf.get(aId) === 'female'), bToA: descTerm(n, true, genderOf.get(bId) === 'female'), score: n }
    return null
  }
  if (n === 0) {
    if (hasAllMaleChildPath(bId, aId)) return { aToB: descTerm(m, false, genderOf.get(aId) === 'female'), bToA: ancTerm(m, false, genderOf.get(bId) === 'female'), score: m }
    if (isBloodDescendant(bId, aId)) return { aToB: descTerm(m, true, genderOf.get(aId) === 'female'), bToA: ancTerm(m, true, genderOf.get(bId) === 'female'), score: m }
    return null
  }

  // 6) 旁系：共同祖先两个孩子 B1/B2（互为同胞），双方各自下延 x/y 代
  //    优先选择互为血亲同胞的 B1/B2（避免父/母双线选错）
  const safeLevelsA = levelsA || []
  const safeLevelsB = levelsB || []
  let firstA = safeLevelsA[m - 1][0]
  let firstB = safeLevelsB[n - 1][0]
  for (const ca of safeLevelsA[m - 1]) {
    for (const cb of safeLevelsB[n - 1]) {
      if (ca === cb) continue
      if (bloodSibling(ca, cb)) { firstA = ca; firstB = cb; break }
    }
    if (firstA !== safeLevelsA[m - 1][0] || firstB !== safeLevelsB[n - 1][0]) break
  }
  const lca = safeLevelsA[m].find((id) => safeLevelsB[n].includes(id)) || ''
  const x = m - 1
  const y = n - 1
  const firstAMale = genderOf.get(firstA) === 'male'
  const firstBMale = genderOf.get(firstB) === 'male'
  const aFemale = genderOf.get(aId) === 'female'
  const bFemale = genderOf.get(bId) === 'female'
  const siblingTerm = (female: boolean) => (female ? '姐妹' : '兄弟')

  // 深方"父系/母系"判定：与支头同支的直接父母（父 → 父系：伯叔/姑；母 → 母系：舅/姨）
  const lineGender = (levels: string[][], sibling: string): string => {
    const reached = (from: string, to: string): boolean => {
      const st: string[] = [from]
      const vs = new Set<string>([from])
      while (st.length) {
        const c = st.pop()!
        if (c === to) return true
        for (const ch of childrenOf.get(c) || []) { if (!vs.has(ch)) { vs.add(ch); st.push(ch) } }
      }
      return false
    }
    const cand = (levels[1] || []).find((p) => p === sibling || reached(sibling, p))
    return genderOf.get(cand || '') || ''
  }
  const parentSide = (levels: string[][], sibling: string): boolean => lineGender(levels, sibling) === 'male'

  // 表亲类型：由双方"通往共同祖先的直接父母线"与支头性别共同决定（从 A 视角）
  //   两线皆父系（且祖源为男）→ 堂；A 父系/B 母系 → 姑表；A 母系/B 父系 → 舅表；两线皆母系 → 姨表
  const mirrorKind = (k: string) => (k === '姑表' ? '舅表' : k === '舅表' ? '姑表' : k)
  const qualifierA = x >= 1 && y >= 1 ? (firstAMale && firstBMale && genderOf.get(lca) === 'male' ? '堂' : lineGender(safeLevelsA, firstA) === 'female' ? (lineGender(safeLevelsB, firstB) === 'female' ? '姨表' : '舅表') : lineGender(safeLevelsB, firstB) === 'female' ? '姑表' : '表') : ''
  const qualifierB = mirrorKind(qualifierA)

  // 表/堂"同级接头"优先：浅方（或其子代）与深方父辈是表/堂兄弟(姐妹)时，按表(堂)线计算。
  // 例：张宏权(浅) × 徐栀意(深)：宏权之子启华 与 栀意之父佳利 是表兄弟 →
  //     宏权 是 佳利 的 表伯 → 宏权 是 栀意 的 表伯祖父；栀意 是 宏权 的 表侄孙女
  if (depth < 2 && (x === 0 ? y : x) >= 2) {
    const shallowIsA = x === 0
    const shallow = shallowIsA ? aId : bId
    const deepParents = shallowIsA ? safeLevelsB[1] : safeLevelsA[1]
    const deepBelow = (shallowIsA ? n : m) - 2
    const shallowFemale = shallowIsA ? aFemale0 : bFemale0
    const deepFemale = shallowIsA ? bFemale0 : aFemale0
    const candidates = [shallow, ...(childrenOf.get(shallow) || [])]
    for (const cP of candidates) {
      for (const dp of deepParents) {
        const rc = computeKinshipRaw(cP, dp, nodes, edges, depth + 1, seenNext)
        if (rc && /^(堂|姨表|舅表|姑表|表)?(兄弟|姐妹)$/.test(rc.aToB)) {
          const q = /^堂/.test(rc.aToB) ? '堂' : /^(姨表|舅表|姑表)/.exec(rc.aToB)?.[1] || '表'
          const dpFemale = genderOf.get(dp) === 'female'
          // 长辈旁系角色：母系线 → 舅/姨；父系线 → 伯/姑
          const shallowRole = shallowFemale ? (dpFemale ? q + '姨' : q + '姑') : dpFemale ? q + '舅' : q + '伯'
          // 代数：浅方若为表亲本人（非其子代），到深方的层差少一层
          const gap = deepBelow + (cP === shallow ? 0 : 1)
          let elder = shallowRole
          if (gap >= 2) elder += ANC_CHAIN[Math.min(gap, 9)]
          elder += shallowFemale ? '母' : '父'
          const junior = q + (dpFemale ? '外甥' : '侄') + (gap >= 2 ? DESC_CHAIN[Math.min(gap, 9)] : '') + (deepFemale ? '女' : '')
          const score = 3 + deepBelow
          return shallowIsA ? { aToB: elder, bToA: junior, score } : { aToB: junior, bToA: elder, score }
        }
      }
    }
  }

  // 深方"父系/母系"判定重复块已合并至上方 lineGender/parentSide

  if (x === y) {
    if (x === 0) return { aToB: siblingTerm(aFemale), bToA: siblingTerm(bFemale), score: m + n }
    if (x === 1) return { aToB: qualifierA + siblingTerm(aFemale), bToA: qualifierB + siblingTerm(bFemale), score: m + n }
    const label = `第${CN[Math.min(x - 2, CN.length - 1)]}代${qualifierA}亲`
    return { aToB: label + (aFemale ? '（女）' : ''), bToA: label + (bFemale ? '（女）' : ''), score: m + n }
  }
  // A 深 B 浅：A 是 B 的 侄/甥系；B 是 A 的长辈旁系
  if (x > y) {
    const diff = x - y
    const prefix = x >= 1 && y >= 1 ? qualifierA : ''
    const prefixB = x >= 1 && y >= 1 ? qualifierB : ''
    // 侄/甥基准：A 方链条中与 B 同代的接续人（如"表兄弟"本人）
    const baseNode = safeLevelsA[y]?.[0] ?? firstA
    const base = genderOf.get(baseNode) === 'male' ? '侄' : '外甥'
    let aToB = ''
    if (diff === 1) aToB = prefix + (aFemale ? `${base}女` : `${base}子`)
    else aToB = prefix + `${base}${DESC_CHAIN[Math.min(diff, 9)]}` + (aFemale ? '女' : '')
    let bToA = ''
    if (prefixB) bToA = bFemale ? '姑' : '伯'
    else if (!bFemale) bToA = parentSide(safeLevelsA, firstA) ? '伯叔' : '舅'
    else bToA = parentSide(safeLevelsA, firstA) ? '姑' : '姨'
    if (diff >= 2) {
      bToA += ANC_CHAIN[Math.min(diff, 9)]
      bToA += bFemale ? '母' : '父'
    } else if (bFemale && bToA !== '伯叔' && bToA !== '舅' && bToA !== '伯') {
      bToA += '母'
    }
    return { aToB, bToA: prefixB + bToA, score: m + n }
  }
  // A 浅 B 深：A 是 B 的长辈旁系；B 是 A 的 侄/甥系
  const diff = y - x
  const prefix = x >= 1 && y >= 1 ? qualifierA : ''
  const prefixB = x >= 1 && y >= 1 ? qualifierB : ''
  let aToB = ''
  if (prefix) aToB = aFemale ? '姑' : '伯'
  else if (!aFemale) aToB = parentSide(safeLevelsB, firstB) ? '伯叔' : '舅'
  else aToB = parentSide(safeLevelsB, firstB) ? '姑' : '姨'
  if (diff >= 2) {
    aToB += ANC_CHAIN[Math.min(diff, 9)]
    aToB += aFemale ? '母' : '父'
  } else if (aFemale && aToB !== '伯叔' && aToB !== '舅' && aToB !== '伯') {
    aToB += '母'
  }
  const bBaseNode = safeLevelsB[x]?.[0] ?? firstB
  const bBase = genderOf.get(bBaseNode) === 'male' ? '侄' : '外甥'
  let bToA = ''
  if (diff === 1) bToA = prefixB + (bFemale ? `${bBase}女` : `${bBase}子`)
  else bToA = prefixB + `${bBase}${DESC_CHAIN[Math.min(diff, 9)]}` + (bFemale ? '女' : '')
  return { aToB: prefix + aToB, bToA, score: m + n }
}

function inLawDescTerm(depth: number, kind: string, female: boolean): string {
  const isXu = kind ? kind === 'xu' : !female
  if (depth === 1) return isXu ? '女婿' : '儿媳'
  if (depth <= 9) return DESC_CHAIN[depth] + (isXu ? '婿' : '媳')
  return `第${CN[Math.min(depth - 10, CN.length - 1)]}世孙` + (isXu ? '婿' : '媳')
}

function inLawAncTerm(depth: number, kind: string, femaleAncestor: boolean): string {
  if (depth === 1) {
    const isXu = kind ? kind === 'xu' : !femaleAncestor
    return isXu ? (femaleAncestor ? '岳母' : '岳父') : femaleAncestor ? '婆婆' : '公公'
  }
  return ancTerm(depth, false, femaleAncestor)
}

function findLCA(levelsA: string[][], levelsB: string[][]): { la: number; lb: number } | null {
  let best: { la: number; lb: number } | null = null
  for (let la = 0; la < levelsA.length; la++) {
    for (let lb = 0; lb < levelsB.length; lb++) {
      if (best && la + lb >= best.la + best.lb) continue
      const has = levelsA[la].some((id) => levelsB[lb].includes(id))
      if (has) best = { la, lb }
    }
  }
  return best
}

/** 纯血缘祖先层级 */
function ancestorLevels(id: string, parentsOf: Map<string, string[]>, maxDepth: number): string[][] | null {
  const levels: string[][] = []
  const seen = new Set<string>()
  let frontier: { id: string; d: number }[] = [{ id, d: 0 }]
  seen.add(id)
  while (frontier.length) {
    const minD = Math.min(...frontier.map((f) => f.d))
    if (minD > maxDepth) break
    const levelIds = frontier.filter((f) => f.d === minD).map((f) => f.id)
    const levelSet = new Set<string>(levelIds)
    for (const l of levelSet) seen.add(l)
    while (levels.length < minD + 1) levels.push([])
    levels[minD].push(...Array.from(levelSet))
    const next: { id: string; d: number }[] = []
    for (const c of levelSet) {
      for (const p of parentsOf.get(c) || []) {
        if (!seen.has(p)) { seen.add(p); next.push({ id: p, d: minD + 1 }) }
      }
    }
    frontier = next
  }
  return levels
}

/** 返回 { aToB: "A 是 B 的 X", bToA: "B 是 A 的 Y" }；无法计算时返回 null。双方向择优保证正反向一致。 */
export function computeKinship(aId: string, bId: string, nodes: KinNode[], edges: KinEdge[]): { aToB: string; bToA: string } | null {
  const r1 = computeKinshipRaw(aId, bId, nodes, edges)
  const r2 = computeKinshipRaw(bId, aId, nodes, edges)
  if (!r1 && !r2) return null
  if (!r1) return { aToB: r2!.bToA, bToA: r2!.aToB }
  if (!r2) return { aToB: r1.aToB, bToA: r1.bToA }
  if (r1.score <= r2.score) return { aToB: r1.aToB, bToA: r1.bToA }
  return { aToB: r2.bToA, bToA: r2.aToB }
}
