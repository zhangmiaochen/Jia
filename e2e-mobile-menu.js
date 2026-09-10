const path = require('path')
const crypto = require('crypto')
const { chromium } = require('C:/Users/19230/AppData/Local/OpenAI/Codex/runtimes/cua_node/b474a88d5d105afa/bin/node_modules/playwright-core')
const b64u = (obj) => Buffer.from(JSON.stringify(obj)).toString('base64url')
const header = b64u({ alg: 'HS256', typ: 'JWT' })
const payload = b64u({ sub: '17e1b0310f67d3df3caea1994455a475', exp: Math.floor(Date.now() / 1000) + 3600 })
const sig = crypto.createHmac('sha256', 'jia-development-secret-change-me').update(header + '.' + payload).digest('base64url')
const token = header + '.' + payload + '.' + sig

const probe = () => {
  const box = (el) => { if (!el) return null; const r = el.getBoundingClientRect(); return { x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height) } }
  const sidebar = document.querySelector('.sidebar')
  const backdrop = document.querySelector('.menu-backdrop')
  const bottom = document.querySelector('.mobile-bottom')
  const label = document.querySelector('.sidebar .nav button span:not(.nav-icon)')
  const brandCopy = document.querySelector('.sidebar .brand-copy')
  const cs = sidebar ? getComputedStyle(sidebar) : null
  return {
    sidebar: sidebar ? { open: sidebar.classList.contains('open'), display: cs.display, visibility: cs.visibility, transform: cs.transform, zIndex: cs.zIndex, box: box(sidebar), navLabelDisplay: label ? getComputedStyle(label).display : null, brandCopyDisplay: brandCopy ? getComputedStyle(brandCopy).display : null, navButtons: document.querySelectorAll('.sidebar .nav button').length } : null,
    backdrop: backdrop ? { box: box(backdrop), zIndex: getComputedStyle(backdrop).zIndex } : null,
    bottomBar: bottom ? { display: getComputedStyle(bottom).display, box: box(bottom) } : null,
    bodyOverflow: document.body.style.overflow,
    scrollY: Math.round(window.scrollY),
    hScroll: { scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth },
  }
}
const ok = (cond, msg) => console.log(`${cond ? 'PASS' : 'FAIL'} :: ${msg}`)
let failures = 0
const check = (cond, msg) => { if (!cond) failures++; ok(cond, msg) }

;(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Users/19230/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-gpu'] })
  const page = await browser.newPage({ viewport: { width: 522, height: 1226 } })
  await page.goto('http://localhost:5173/', { waitUntil: 'domcontentloaded', timeout: 30000 })
  await page.evaluate((t) => localStorage.setItem('jia_token', t), token)
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(3000)

  console.log('=== 1. 小屏 522x1226：初始关闭 ===')
  let s = await page.evaluate(probe)
  console.log(JSON.stringify(s))
  check(s.sidebar.open === false && s.sidebar.visibility === 'hidden' && s.sidebar.box.x < 0, '抽屉初始隐藏且位于屏幕左侧外')
  check(s.backdrop === null, '未打开时没有遮罩')
  check(s.hScroll.scrollWidth <= s.hScroll.clientWidth, '关闭状态下没有横向滚动条')

  console.log('=== 2. 点击左上角 ☰ 展开 ===')
  await page.click('.mobile-menu')
  await page.waitForTimeout(700)
  s = await page.evaluate(probe)
  console.log(JSON.stringify(s))
  check(s.sidebar.open === true && s.sidebar.visibility === 'visible', '抽屉可见（不再被 display:none 吃掉）')
  check(s.sidebar.box.x === 0 && s.sidebar.box.w > 200, `抽屉贴左显示，宽 ${s.sidebar.box.w}px`)
  check(s.sidebar.navLabelDisplay !== 'none', '菜单文字标签重新显示（未被 1100px 图标栏规则隐藏）')
  check(s.sidebar.brandCopyDisplay === 'grid', '品牌文字块显示')
  check(s.sidebar.navButtons === 9, `抽屉内包含全部 ${s.sidebar.navButtons} 个导航项`)
  check(!!s.backdrop, '出现遮罩层')
  check(s.sidebar.box.y === 64, `抽屉从顶栏下方滑出（top=${s.sidebar.box.y}），不遮挡 ☰ 按钮`)
  const hit = await page.evaluate(() => { const b = document.querySelector('.mobile-menu').getBoundingClientRect(); const el = document.elementFromPoint(b.x + b.width / 2, b.y + b.height / 2); return el ? (el.className || el.tagName) : 'none' })
  check(String(hit).includes('mobile-menu'), `展开状态下 ☰ 仍是最上层可点元素（命中：${hit}）`)
  check(s.bodyOverflow === 'hidden', '抽屉打开时锁定背景滚动')
  await page.screenshot({ path: path.join(__dirname, 'mmenu-open.png') })

  console.log('=== 3. 再次点击 ☰ 收起 ===')
  await page.click('.mobile-menu')
  await page.waitForTimeout(700)
  s = await page.evaluate(probe)
  check(s.sidebar.open === false && s.sidebar.visibility === 'hidden' && s.backdrop === null, '再次点击可收起抽屉并移除遮罩')
  check(s.bodyOverflow === '', '收起后恢复背景滚动')

  console.log('=== 4. Esc 关闭 ===')
  await page.click('.mobile-menu')
  await page.waitForTimeout(600)
  await page.keyboard.press('Escape')
  await page.waitForTimeout(600)
  s = await page.evaluate(probe)
  check(s.sidebar.open === false && s.backdrop === null, '按 Esc 可收起抽屉')

  console.log('=== 5. 点击遮罩关闭 ===')
  await page.click('.mobile-menu')
  await page.waitForTimeout(600)
  await page.mouse.click(480, 900)
  await page.waitForTimeout(600)
  s = await page.evaluate(probe)
  check(s.sidebar.open === false && s.backdrop === null, '点击遮罩空白处可收起抽屉')

  console.log('=== 6. 抽屉内点击「人物」导航 ===')
  await page.click('.mobile-menu')
  await page.waitForTimeout(600)
  const clicked = await page.evaluate(() => {
    const sidebar = document.querySelector('.sidebar')
    if (!sidebar || getComputedStyle(sidebar).visibility === 'hidden') return 'sidebar-not-clickable'
    const btn = [...sidebar.querySelectorAll('.nav button')].find((b) => b.textContent.includes('人物'))
    if (!btn || btn.getBoundingClientRect().width === 0) return 'button-not-rendered'
    btn.click()
    return 'clicked'
  })
  await page.waitForTimeout(1800)
  s = await page.evaluate(probe)
  console.log('nav =', clicked, '| url =', new URL(page.url()).pathname)
  check(clicked === 'clicked', '抽屉展开后导航项可点击')
  check(new URL(page.url()).pathname === '/persons', '导航跳转到 /persons')
  check(s.sidebar.open === false && s.backdrop === null, '导航后抽屉自动收起')
  check(await page.evaluate(() => !!document.querySelector('.person-list-head')), '人物页面正常渲染')
  await page.screenshot({ path: path.join(__dirname, 'mmenu-nav.png') })

  console.log('=== 7. 背景滚动锁定实测 ===')
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)
  await page.evaluate(() => { const d = document.createElement('div'); d.id = 'e2e-spacer'; d.style.height = '1600px'; document.querySelector('.main').appendChild(d) })
  await page.evaluate(() => window.scrollTo(0, 0))
  const scrollable = await page.evaluate(() => document.scrollingElement.scrollHeight - window.innerHeight)
  check(scrollable > 100, `测试页可滚动区域 ${scrollable}px（用于验证锁定是否真实生效）`)
  await page.mouse.move(250, 240); await page.mouse.wheel(0, 600); await page.waitForTimeout(400)
  const scrolledBeforeOpen = (await page.evaluate(probe)).scrollY
  check(scrolledBeforeOpen > 0, `未打开抽屉时页面可正常滚动（scrollY=${scrolledBeforeOpen}）`)
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.click('.mobile-menu')
  await page.waitForTimeout(700)
  await page.mouse.move(480, 900); await page.mouse.wheel(0, 600); await page.waitForTimeout(400)
  const scrolledWhileOpen = (await page.evaluate(probe)).scrollY
  check(scrolledWhileOpen === 0, `抽屉打开时背景被锁定，滚轮无效（scrollY=${scrolledWhileOpen}）`)
  await page.mouse.click(480, 900)
  await page.waitForTimeout(600)
  await page.mouse.move(250, 240); await page.mouse.wheel(0, 600); await page.waitForTimeout(400)
  const scrolledAfterClose = (await page.evaluate(probe)).scrollY
  check(scrolledAfterClose > 0, `收起后背景滚动恢复（scrollY=${scrolledAfterClose}）`)
  await page.evaluate(() => document.getElementById('e2e-spacer')?.remove())

  console.log('=== 8. 展开状态下窗口变宽（横屏/桌面） ===')
  await page.setViewportSize({ width: 522, height: 1226 })
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.click('.mobile-menu')
  await page.waitForTimeout(600)
  check((await page.evaluate(probe)).sidebar.open === true, '小屏下已展开')
  await page.setViewportSize({ width: 1280, height: 900 })
  await page.waitForTimeout(900)
  s = await page.evaluate(probe)
  console.log(JSON.stringify(s))
  check(s.sidebar.open === false && s.backdrop === null, '变宽后抽屉自动收起，无残留遮罩')
  check(s.bodyOverflow === '', '变宽后背景滚动未被锁死')
  check(s.sidebar.box.w === 256 && s.sidebar.transform === 'none', '变宽后回到常驻 256px 侧栏')

  console.log('=== 9. 平板 900px：图标栏模式不回归 ===')
  await page.setViewportSize({ width: 900, height: 1100 })
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)
  s = await page.evaluate(probe)
  console.log(JSON.stringify(s))
  check(s.sidebar.display === 'flex' && s.sidebar.box.x === 0 && s.sidebar.box.w === 82, `900px 仍是常驻图标栏（宽 ${s.sidebar.box.w}px）`)
  check(s.sidebar.transform === 'none', '900px 下未应用抽屉位移')
  check(s.bottomBar.display === 'none', '900px 下底部导航栏不再出现')

  console.log('=== 10. 桌面 1280px：侧栏常驻 + 无移动端残留 ===')
  await page.setViewportSize({ width: 1280, height: 1200 })
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(2500)
  s = await page.evaluate(probe)
  console.log(JSON.stringify(s))
  check(s.sidebar.display === 'flex' && s.sidebar.box.x === 0 && s.sidebar.box.w === 256, `1280px 侧栏常驻 256px（实际 ${s.sidebar.box.w}px）`)
  check(s.sidebar.navLabelDisplay !== 'none' && s.sidebar.visibility === 'visible', '1280px 菜单文字正常显示')
  check(s.backdrop === null, '1280px 无遮罩层')
  check(s.bottomBar.display === 'none', '1280px 底部移动导航栏隐藏')
  check(s.hScroll.scrollWidth <= s.hScroll.clientWidth, '1280px 无横向溢出')
  await page.screenshot({ path: path.join(__dirname, 'mmenu-desktop.png') })

  console.log(failures === 0 ? '\nALL CHECKS PASSED' : `\n${failures} CHECK(S) FAILED`)
  await browser.close()
  if (failures) process.exit(1)
})().catch((e) => { console.error('FATAL', e); process.exit(1) })
