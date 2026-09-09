const crypto = require('crypto')
const { chromium } = require('C:/Users/19230/AppData/Local/OpenAI/Codex/runtimes/cua_node/b474a88d5d105afa/bin/node_modules/playwright-core')
const b64u = (obj) => Buffer.from(JSON.stringify(obj)).toString('base64url')
const header = b64u({ alg: 'HS256', typ: 'JWT' })
const payload = b64u({ sub: '17e1b0310f67d3df3caea1994455a475', exp: Math.floor(Date.now() / 1000) + 3600 })
const sig = crypto.createHmac('sha256', 'jia-development-secret-change-me').update(header + '.' + payload).digest('base64url')
const token = header + '.' + payload + '.' + sig
;(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Users/19230/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-gpu'] })
  const page = await browser.newPage({ viewport: { width: 1280, height: 1200 } })
  await page.addInitScript((t) => { localStorage.setItem('jia_token', t) }, token)
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(4000)
  const readList = () => page.evaluate(() => Array.from(document.querySelectorAll('.relation-list .member span')).map((s) => s.textContent.trim()).slice(0, 6))
  const famSelect = () => page.evaluate(() => {
    const sel = document.querySelector('.family-select select')
    const opts = Array.from(sel.options).map((o) => ({ v: o.value, t: o.textContent.trim() }))
    return { value: sel.value, opts }
  })
  console.log('FAMILIES:', JSON.stringify(await famSelect()))
  console.log('LIST@张氏:', JSON.stringify(await readList()))
  // 切换到 验证家族（有2条关系）
  const families = await famSelect()
  const target = families.opts.find((o) => o.t.includes('验证家族'))
  if (target) {
    await page.selectOption('.family-select select', target.v)
    await page.waitForTimeout(3000)
    console.log('SELECTED:', target.t, '->', JSON.stringify(await famSelect()))
    console.log('LIST@验证家族:', JSON.stringify(await readList()))
  } else {
    console.log('no 验证家族 option found')
  }
  await browser.close()
})().catch((e) => { console.error('FATAL', e); process.exit(1) })
