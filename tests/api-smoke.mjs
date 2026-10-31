import assert from 'node:assert/strict'
import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { randomBytes } from 'node:crypto'

const base = process.env.TEST_URL || 'http://127.0.0.1:8081'
const password = (await readFile(process.env.TEST_PASSWORD_FILE || '/run/bootstrap_password', 'utf8')).trim()
const results = []
class Client {
  cookie = ''
  csrf = ''
  async request(path, { method = 'GET', body, csrf = true, origin } = {}) {
    const headers = { 'Content-Type': 'application/json' }
    if (this.cookie) headers.Cookie = this.cookie
    if (csrf) headers['X-CSRF-Token'] = this.csrf
    if (origin) headers.Origin = origin
    const response = await fetch(`${base}/api${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
    const cookie = response.headers.getSetCookie()[0]
    if (cookie) this.cookie = cookie.split(';')[0]
    const data = await response.json()
    return { response, data }
  }
  async login(username, password) {
    const result = await this.request('/login', { method: 'POST', body: { username, password } })
    assert.equal(result.response.status, 200)
    this.csrf = result.data.csrfToken
    return result
  }
}
function passed(name) { results.push({ name, passed: true }); console.log(`PASS ${name}`) }
const guest = new Client()
assert.equal((await guest.request('/preferences')).response.status, 401)
assert.equal((await guest.request('/admin/users')).response.status, 401)
passed('Anonymous access denied')
assert.equal((await guest.request('/login', { method:'POST', body:{username:'admin',password:'invalid-password'} })).response.status, 401)
passed('Invalid password denied')
const admin = new Client()
const login = await admin.login('admin', password)
assert.match(login.response.headers.get('set-cookie'), /HttpOnly/i)
assert.match(login.response.headers.get('set-cookie'), /SameSite=Strict/i)
assert.equal(login.data.user.role, 'admin')
passed('Administrator login with HttpOnly / SameSite session')
const original = structuredClone((await admin.request('/preferences')).data)
assert.equal((await admin.request('/preferences', { method:'PUT', body:original, csrf:false })).response.status, 403)
assert.equal((await admin.request('/preferences', { method:'PUT', body:original, origin:'https://attacker.example' })).response.status, 403)
passed('CSRF and cross-origin writes denied')
assert.equal((await admin.request('/preferences', { method:'PUT', body:{ ...original, windows:[{id:'shell-exec',x:40,y:40,width:400,height:300,minimized:false,maximized:false}] } })).response.status, 400)
passed('Unknown app rejected in persisted window state')
const system = (await admin.request('/system')).data
assert.equal(system.database, 'connected')
assert.ok(system.heapBytes > 0)
assert.ok(system.diskTotalBytes > 0)
assert.equal(system.integrations, undefined)
passed('Real system status, no placeholder integrations')
const testUsername = `e2e_${randomBytes(5).toString('hex')}`
const testPassword = randomBytes(20).toString('hex')
try {
  const changed = { ...original, notes:'API isolation test — admin private', wallpaper:'sage', mode:'windows', windows:[] }
  assert.equal((await admin.request('/preferences', { method:'PUT', body:changed })).response.status, 200)
  assert.equal((await admin.request('/preferences')).data.mode, 'windows')
  const created = await admin.request('/admin/users', { method:'POST', body:{username:testUsername,displayName:'E2E isolation test',password:testPassword,role:'user'} })
  assert.equal(created.response.status, 201)
  assert.equal('password' in created.data, false)
  await mkdir('artifacts', { recursive:true })
  await writeFile('artifacts/test-user.txt', testUsername)
  const user = new Client()
  await user.login(testUsername,testPassword)
  assert.equal((await user.request('/preferences')).data.notes, '')
  assert.equal((await user.request('/preferences')).data.mode, 'focus')
  assert.equal((await user.request('/admin/users')).response.status, 403)
  assert.equal((await user.request('/admin/users', { method:'POST',body:{} })).response.status, 403)
  passed('Standard user denied administrator API')
  const userPrefs = (await user.request('/preferences')).data
  userPrefs.notes = 'API isolation test — user private'
  userPrefs.wallpaper = 'midnight'
  assert.equal((await user.request('/preferences', { method:'PUT',body:userPrefs })).response.status, 200)
  assert.equal((await admin.request('/preferences')).data.notes, changed.notes)
  assert.equal((await user.request('/preferences')).data.notes, userPrefs.notes)
  const reLogin = new Client()
  await reLogin.login(testUsername,testPassword)
  assert.equal((await reLogin.request('/preferences')).data.notes, userPrefs.notes)
  passed('Preferences isolated across users and preserved across sessions')
  const audit = (await user.request('/audit')).data
  assert.ok(audit.every(entry => entry.actor === testUsername))
  passed('Standard user audit isolated')
  await user.request('/logout',{method:'POST'})
  assert.equal((await user.request('/session')).response.status, 401)
  passed('Logout revokes server-side session')
  await reLogin.request('/logout',{method:'POST'})
} finally {
  assert.equal((await admin.request('/preferences', {method:'PUT',body:original})).response.status, 200)
}
await admin.request('/logout', {method:'POST'})
await writeFile('artifacts/api-smoke.json',JSON.stringify({at:new Date().toISOString(),results},null,2))
console.log(`Completed ${results.length} checks. Disposable test account recorded for database cleanup.`)
