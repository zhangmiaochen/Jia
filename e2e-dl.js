const fs = require('fs')
const path = require('path')
const crypto = require('crypto')
const { chromium } = require('C:/Users/19230/AppData/Local/OpenAI/Codex/runtimes/cua_node/b474a88d5d105afa/bin/node_modules/playwright-core')
const b64u = (obj) => Buffer.from(JSON.stringify(obj)).toString('base64url')
const header = b64u({ alg: 'HS256', typ: 'JWT' })
const payload = b64u({ sub: '17e1b0310f67d3df3caea1994455a475', exp: Math.floor(Date.now() / 1000) + 3600 })
const sig = crypto.createHmac('sha256', 'jia-development-secret-change-me').update(header + '.' + payload).digest('base64url')
const token = header + '.' + payload + '.' + sig
;(async () => {
  const browser = await chromium.launch({ executablePath: 'C:/Users/19230/AppData/Local/ms-playwright/chromium-1228/chrome-win64/chrome.exe', headless: true, args: ['--no-sandbox', '--disable-gpu'] })
  const page = await browser.newPage({ viewport: { width: 1280, height: 1400 } })
  await page.goto('http://localhost:5173/', { waitUntil: 'domcontentloaded', timeout: 30000 })
  await page.evaluate((t) => localStorage.setItem('jia_token', t), token)
  await page.goto('http://localhost:5173/graph', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(4500)
  const [download] = await Promise.all([
    page.waitForEvent('download', { timeout: 30000 }),
    page.click('button:has-text("下载图片")'),
  ])
  const filePath = path.join(__dirname, 'graph-export.png')
  await download.saveAs(filePath)
  const buf = fs.readFileSync(filePath)
  const isPng = buf[0] === 0x89 && buf[1] === 0x50 && buf[2] === 0x4e && buf[3] === 0x47
  console.log('downloaded:', download.suggestedFilename(), 'size:', buf.length, 'PNG signature:', isPng)
  await browser.close()
})().catch((e) => { console.error('FATAL', e); process.exit(1) })
