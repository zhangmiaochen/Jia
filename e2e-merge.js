// 家族合并向导的端到端验证：
// - 浏览器加载用户的 vite 开发服务器（localhost:5173，含最新前端代码）
// - /api/v1 请求经 Playwright 代理转发到隔离后端（127.0.0.1:8099，全新临时库），不碰开发库
// - 两个账号分别建了「同一个家族」，走完整合并向导并由断言核对结果
const path = require('path')
const { chromium } = require('C:/Users/19230/AppData/Local/OpenAI/Codex/runtimes/cua_node/b474a88d5d105afa/bin/node_modules/playwright-core')

const API = 'http://127.0.0.1:8099/api/v1'
let failures = 0
const ok = (cond, msg) => { if (!cond) failures++; console.log(`${cond ? 'PASS' : 'FAIL'} :: ${msg}`) }

async function call(method, url, token, body) {
  const res = await fetch(API + url, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const json = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(`${method} ${url} -> ${res.status} ${JSON.stringify(json.error || json)}`)
  return json.data
}

async function seed() {
  const run = Date.now().toString(36)
  const a = await call('POST', '/auth/register', '', { email: `owner-a-${run}@example.com`, password: 'secret123', displayName: '甲方' })
  const b = await call('POST', '/auth/register', '', { email: `owner-b-${run}@example.com`, password: 'secret123', displayName: '乙方' })
  const familyA = await call('POST', '/families', a.token, { name: '徐氏家族', surname: '徐', origin: '浙江绍兴' })
  const familyB = await call('POST', '/families', b.token, { name: '徐氏家谱（乙方建的）', surname: '徐' })

  const person = (token, family, payload) => call('POST', `/families/${family}/persons`, token, payload)
  const relate = (token, from, payload) => call('POST', `/persons/${from}/relations`, token, payload)

  const fuA = await person(a.token, familyA.id, { name: '徐父', gender: 'male', birthDate: '1950-01-01' })
  const shuA = await person(a.token, familyA.id, { name: '徐书帆', gender: 'male', birthDate: '1980-05-06', occupation: '教师' })
  const ningA = await person(a.token, familyA.id, { name: '张峪宁', gender: 'female', birthDate: '1985-08-08' })
  await relate(a.token, fuA.id, { toPersonID: shuA.id, relationType: 'FATHER' })
  await relate(a.token, shuA.id, { toPersonID: ningA.id, relationType: 'HUSBAND' })

  const shuB = await person(b.token, familyB.id, { name: '徐书帆', gender: 'male', birthDate: '1980-05-06' })
  const mingB = await person(b.token, familyB.id, { name: '徐小明', gender: 'male', birthDate: '2010-01-01' })
  await relate(b.token, shuB.id, { toPersonID: mingB.id, relationType: 'FATHER' })

  const genA = await call('POST', `/families/${familyA.id}/genealogies`, a.token, { name: '徐氏族谱' })
  await call('POST', `/genealogies/${genA.id}/persons`, a.token, { person_id: shuA.id })
  const genB = await call('POST', `/families/${familyB.id}/genealogies`, b.token, { name: '徐氏族谱' })
  await call('POST', `/genealogies/${genB.id}/persons`, b.token, { person_id: mingB.id })
  const bookB = await call('POST', `/families/${familyB.id}/books`, b.token, { title: '徐书帆生平', rootPersonID: shuB.id })

  const invite = await call('POST', `/families/${familyA.id}/merge-invites`, a.token, undefined)
  return { a, b, familyA, familyB, shuA, shuB, mingB, bookB, code: invite.token }
}

;(async () => {
  const data = await seed()
  console.log(`已准备：A=${data.familyA.name}（3 人）、B=${data.familyB.name}（2 人，含重复的徐书帆），合并码 ${data.code}`)

  const browser = await chromium.launch({ executablePath: 'C:/Users/19230/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-gpu'] })
  const page = await browser.newPage({ viewport: { width: 1280, height: 1000 } })

  // 把前端的 API 请求代理到隔离后端（浏览器侧同源/CORS 都不受影响）
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const headers = { ...request.headers() }
    delete headers.host; delete headers['content-length']; delete headers['accept-encoding']
    const res = await fetch(`http://127.0.0.1:8099${url.pathname}${url.search}`, {
      method: request.method(), headers,
      body: ['GET', 'HEAD'].includes(request.method()) ? undefined : request.postDataBuffer(),
    })
    const out = {}
    for (const [key, value] of res.headers) {
      if (!['content-encoding', 'content-length', 'transfer-encoding'].includes(key.toLowerCase())) out[key] = value
    }
    route.fulfill({ status: res.status, headers: out, body: Buffer.from(await res.arrayBuffer()) })
  })

  // ---------- 以「乙方」身份进入家族资料页 ----------
  await page.goto('http://localhost:5173/', { waitUntil: 'domcontentloaded', timeout: 30000 })
  await page.evaluate((token) => { localStorage.setItem('jia_token', token); localStorage.removeItem('jia_family') }, data.b.token)
  await page.goto('http://localhost:5173/family', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)

  ok(await page.locator('text=家族合并').count() > 0, '家族资料页出现「家族合并」入口')
  ok(await page.locator('h2:has-text("徐氏家谱")').count() > 0, `当前家族显示为乙方建的家族`)

  // ---------- 打开向导 ----------
  await page.click('button:has-text("开始合并")')
  await page.waitForTimeout(600)
  ok(await page.locator('text=我是主家族，让别人并进来').count() > 0, '向导第一步展示两种方式')
  await page.screenshot({ path: path.join(__dirname, 'merge-1-menu.png') })

  // ---------- 输入合并码 → 校验 ----------
  await page.click('button:has-text("输入合并码")')
  await page.waitForTimeout(400)
  await page.fill('.merge-inline input', data.code)
  await page.click('button:has-text("校验")')
  await page.waitForTimeout(1200)
  ok(await page.locator('.merge-target strong:has-text("徐氏家族")').count() > 0, '校验后显示对方的主家族「徐氏家族」')
  ok(await page.locator('text=合并码有效').count() > 0, '显示合并码有效标记')

  // ---------- 人物对照 ----------
  await page.click('button:has-text("下一步：人物对照")')
  await page.waitForTimeout(1500)
  const rows = await page.locator('.merge-pair-row').count()
  ok(rows === 2, `人物对照列出被并入家族的 2 位人物（实际 ${rows}）`)
  const suggested = await page.locator('.merge-suggest').first().textContent()
  ok(/建议：与「徐书帆」合并/.test(suggested || ''), `系统给出配对建议：${(suggested || '').trim()}`)
  const selected = await page.locator('.merge-pair-row select, .merge-pair-row .arco-select-view-value').first().textContent()
  ok(/徐书帆/.test(selected || ''), `建议配对已预选：${(selected || '').trim()}`)
  ok(await page.locator('text=两边同名的族谱合并成一份').count() > 0, '识别出两边同名的族谱并提供合并选项')
  await page.screenshot({ path: path.join(__dirname, 'merge-2-pair.png') })

  // ---------- 确认执行 ----------
  await page.click('button:has-text("下一步：确认执行")')
  await page.waitForTimeout(800)
  const checklist = await page.locator('.merge-checklist').textContent()
  ok(/2 位人物移入接收方/.test(checklist || ''), '确认页说明迁入人数')
  ok(/1<\/b> 位按人物对照合并|1 位按人物对照合并/.test((await page.locator('.merge-checklist').innerHTML()) || ''), '确认页说明有 1 位重复人物会被合并')
  await page.screenshot({ path: path.join(__dirname, 'merge-3-confirm.png') })

  await page.click('button:has-text("确认合并")')
  await page.waitForTimeout(3000)
  ok(await page.locator('text=合并完成').count() > 0, '执行后展示合并完成')
  const stats = await page.locator('.merge-stats').textContent()
  ok(/重复人物合并/.test(stats || ''), `结果面板展示统计：${(stats || '').replace(/\s+/g, ' ').trim().slice(0, 80)}`)
  await page.screenshot({ path: path.join(__dirname, 'merge-4-done.png') })
  await page.click('button:has-text("完成")')
  await page.waitForTimeout(2500)

  // ---------- 合并后的界面状态 ----------
  const options = await page.locator('.family-select select option').allTextContents()
  ok(options.length === 1 && options[0].includes('徐氏家族'), `家族切换器只剩合并后的家族：${options.join(' / ')}`)
  ok(await page.locator('h2:has-text("徐氏家族")').count() > 0, '已自动切换到合并后的家族')
  await page.goto('http://localhost:5173/persons', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)
  const personRows = await page.locator('.person-row').count()
  ok(personRows === 4, `人物列表已包含两边的 4 位人物（实际 ${personRows}）`)
  await page.screenshot({ path: path.join(__dirname, 'merge-5-persons.png') })

  // ---------- 主家族一侧：生成合并码 ----------
  await page.evaluate((token) => { localStorage.setItem('jia_token', token); localStorage.removeItem('jia_family') }, data.a.token)
  await page.goto('http://localhost:5173/family', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)
  await page.click('button:has-text("开始合并")')
  await page.waitForTimeout(500)
  await page.click('button:has-text("生成合并码")')
  await page.waitForTimeout(1500)
  const code = (await page.locator('.merge-code').textContent() || '').trim()
  ok(/^[A-Z2-9]{4}-[A-Z2-9]{4}-[A-Z2-9]{4}$/.test(code), `生成合并码：${code}`)
  ok(await page.locator('button:has-text("复制链接")').count() > 0, '提供复制合并码 / 复制链接入口')
  await page.screenshot({ path: path.join(__dirname, 'merge-6-code.png') })

  // ---------- 小屏（手机宽度）下的合并弹窗 ----------
  await page.setViewportSize({ width: 522, height: 1226 })
  await page.waitForTimeout(800)
  const box = await page.locator('.arco-modal:has(.merge-wizard)').boundingBox()
  ok(Boolean(box) && box.width <= 522, `小屏下弹窗宽度自适应视口：${box ? Math.round(box.width) : 'n/a'}px`)
  ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), '小屏下没有横向滚动条')
  await page.screenshot({ path: path.join(__dirname, 'merge-7-mobile.png') })

  await browser.close()
  console.log(failures === 0 ? '\nALL CHECKS PASSED' : `\n${failures} CHECK(S) FAILED`)
  process.exit(failures ? 1 : 0)
})().catch((error) => { console.error('FATAL', error); process.exit(1) })
