// npm run perf: đo hiệu năng bản build production bằng Lighthouse, cách đo dùng khi tối ưu.
// Build (bỏ qua với --no-build), chạy `vite preview`, đăng nhập tài khoản seed qua API để đo
// trang cần đăng nhập, chạy Lighthouse nhiều lần mỗi trang × profile, in trung vị.
//
// Cần: backend chạy (API_URL, mặc định http://127.0.0.1:8080), dữ liệu seed, SEED_PASSWORD
// (biến môi trường hoặc ../.env), Chrome hay Edge (CHROME_PATH nếu không tự tìm thấy).
// Tuỳ chọn: --runs N (mặc định 3), --no-build, --port P (4180), --pages login,assets,
// PERF_EMAIL (mặc định officer@storeit.test). Không đo dev server: Vite dev không gộp file,
// điểm thấp hơn nhiều và không nói gì về bản deploy.
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

const args = process.argv.slice(2)
const opt = (name, fallback) => {
  const i = args.indexOf(`--${name}`)
  return i >= 0 ? args[i + 1] : fallback
}
const runs = Number(opt('runs', '3'))
const port = Number(opt('port', '4180'))
// tên trang có hay không có / đầu đều được (Git Bash đổi tham số bắt đầu bằng / thành đường dẫn)
const pages = opt('pages', 'login,assets').split(',').map((p) => `/${p.trim().replace(/^\/+/, '')}`)
const api = process.env.API_URL ?? 'http://127.0.0.1:8080'
const email = process.env.PERF_EMAIL ?? 'officer@storeit.test'
const out = join(import.meta.dirname, '..', '.perf')
mkdirSync(out, { recursive: true })

// Không qua shell (khỏi lo đặt ngoặc tham số): vite chạy thẳng bằng node; npx trên Windows
// là npx-cli.js của npm cạnh node.exe, nơi khác là lệnh npx
const vite = join(import.meta.dirname, '..', 'node_modules', 'vite', 'bin', 'vite.js')
const npxCli = join(dirname(process.execPath), 'node_modules', 'npm', 'bin', 'npx-cli.js')

// đợi tiến trình xong; lỗi thì kèm phần cuối stderr để biết vì sao
function wait(p, what) {
  let err = ''
  p.stderr?.on('data', (d) => (err = (err + d).slice(-2000)))
  return new Promise((resolve, reject) => p.on('exit', (code) => (code === 0 ? resolve() : reject(new Error(`${what} exited with ${code}
${err}`)))))
}
const quiet = { stdio: ['ignore', 'ignore', 'pipe'] }
const runVite = (viteArgs) => spawn(process.execPath, [vite, ...viteArgs], quiet)
const runNpx = (npxArgs, env) => {
  const opts = { ...quiet, env: { ...process.env, ...env } }
  const p = process.platform === 'win32' ? spawn(process.execPath, [npxCli, ...npxArgs], opts) : spawn('npx', npxArgs, opts)
  return wait(p, npxArgs[1])
}

function chromePath() {
  if (process.env.CHROME_PATH) return process.env.CHROME_PATH
  const candidates = [
    'C:/Program Files/Google/Chrome/Application/chrome.exe',
    'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
    'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/usr/bin/google-chrome',
    '/usr/bin/chromium',
  ]
  return candidates.find((c) => existsSync(c))
}

function seedPassword() {
  if (process.env.SEED_PASSWORD) return process.env.SEED_PASSWORD
  const env = join(import.meta.dirname, '..', '..', '.env')
  const line = existsSync(env) ? readFileSync(env, 'utf8').split(/\r?\n/).find((l) => l.startsWith('SEED_PASSWORD=')) : undefined
  return line?.slice('SEED_PASSWORD='.length).replace(/^"|"$/g, '')
}

// cookie refresh của một phiên mới (mỗi lần chạy một phiên: refresh token xoay vòng)
async function sessionCookie() {
  const res = await fetch(`${api}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password: seedPassword() }),
  })
  const ck = res.headers.getSetCookie().find((c) => c.startsWith('storeit_refresh='))
  if (!res.ok || !ck) throw new Error(`sign-in as ${email} failed (${res.status}); is the backend running with seed data?`)
  return ck.split(';')[0]
}

async function waitFor(url) {
  for (let i = 0; i < 60; i++) {
    try {
      if ((await fetch(url)).ok) return
    } catch {}
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`${url} did not come up`)
}

const median = (xs) => [...xs].sort((a, b) => a - b)[Math.floor(xs.length / 2)]

const chrome = chromePath()
if (!chrome) throw new Error('no Chrome or Edge found; set CHROME_PATH')
if (!seedPassword()) throw new Error('SEED_PASSWORD not set (environment or ../.env)')

if (!args.includes('--no-build')) {
  console.log('building…')
  await wait(runVite(['build']), 'vite build')
}
const preview = runVite(['preview', '--port', String(port), '--strictPort'])
try {
  await waitFor(`http://localhost:${port}/login`)
  const rows = []
  for (const page of pages) {
    for (const profile of ['mobile', 'desktop']) {
      const results = []
      for (let i = 1; i <= runs; i++) {
        const file = join(out, `${page.replace(/\W+/g, '') || 'root'}-${profile}-${i}.json`)
        const flags = ['--quiet', '--only-categories=performance', `--output=json`, `--output-path=${file}`, '--chrome-flags=--headless=new']
        if (profile === 'desktop') flags.push('--preset=desktop')
        if (page !== '/login') {
          const headers = join(out, 'headers.json')
          writeFileSync(headers, JSON.stringify({ Cookie: await sessionCookie() }))
          flags.push(`--extra-headers=${headers}`)
        }
        await runNpx(['-y', 'lighthouse@12', `http://localhost:${port}${page}`, ...flags], { CHROME_PATH: chrome })
        results.push(JSON.parse(readFileSync(file, 'utf8')))
      }
      const m = (k) => median(results.map((r) => r.audits[k].numericValue))
      rows.push({
        page,
        profile,
        score: median(results.map((r) => Math.round(r.categories.performance.score * 100))),
        FCP: `${(m('first-contentful-paint') / 1000).toFixed(2)} s`,
        LCP: `${(m('largest-contentful-paint') / 1000).toFixed(2)} s`,
        TBT: `${Math.round(m('total-blocking-time'))} ms`,
        CLS: m('cumulative-layout-shift').toFixed(3),
        SI: `${(m('speed-index') / 1000).toFixed(2)} s`,
      })
      console.log(`${page} ${profile} done`)
    }
  }
  console.log(`\nLighthouse, production build, median of ${runs} (reports in .perf/):`)
  console.table(rows)
} finally {
  // dừng vite preview (kèm tiến trình con) và đợi xong, để cổng được trả lại
  if (process.platform === 'win32' && preview.pid) spawnSync('taskkill', ['/pid', String(preview.pid), '/T', '/F'], { stdio: 'ignore' })
  else preview.kill()
}
