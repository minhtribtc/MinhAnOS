import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const base='http://127.0.0.1:8083',password=(await readFile('/run/bootstrap_password','utf8')).trim()
const login=await fetch(base+'/api/login',{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify({username:'admin',password})});assert.equal(login.status,200)
const cookie=login.headers.getSetCookie()[0].split(';')[0],session=await login.json(),headers={Cookie:cookie,'Content-Type':'application/json',Origin:base,'X-CSRF-Token':session.csrfToken}
const me=await fetch(base+'/api/v1/identity/me',{headers});assert.equal(me.status,200);assert.match((await me.json()).id,/^[0-9a-f-]{36}$/);assert(me.headers.get('ETag'))
const roles=await fetch(base+'/api/v1/identity/roles',{headers});assert.equal(roles.status,200);assert.equal((await roles.json()).items.length,3)
assert.equal((await fetch(base+'/api/v1/identity/reauthenticate',{method:'POST',headers,body:JSON.stringify({password})})).status,200)
assert.equal((await fetch(base+'/api/logout',{method:'POST',headers,body:'{}'})).status,200)
console.log('PASS fresh standalone Drive: login assigns public UUID, v1 me/roles/ETag/reauth work without Desktop')
