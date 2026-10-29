import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const base=process.env.DRIVE_URL||'http://10.1.1.20:8082'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const health=await fetch(base+'/api/health');assert.equal(health.status,200)
const html=await(await fetch(base+'/')).text();assert.match(html,/MinhAnOS Drive/);assert.match(html,/assets/)
const response=await fetch(base+'/api/login',{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify({username:'admin',password})});assert.equal(response.status,200)
const cookie=response.headers.getSetCookie()[0].split(';')[0],session=await response.json()
const list=await fetch(base+'/api/files/list?root=personal',{headers:{Cookie:cookie}});assert.equal(list.status,200)
const logout=await fetch(base+'/api/logout',{method:'POST',headers:{Cookie:cookie,'Content-Type':'application/json','X-CSRF-Token':session.csrfToken,Origin:base},body:'{}'});assert.equal(logout.status,200)
console.log('PASS Drive frontend, login, personal files and logout work without any desktop service')
