import assert from 'node:assert/strict'
import {readFile,writeFile,mkdir} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
import {createUser} from './identity-user.mjs'
const base=process.env.TEST_URL||'http://10.1.1.20:8081',drive=process.env.DRIVE_URL||'http://10.1.1.20:8082'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
class Client{cookie='';csrf='';constructor(base){this.base=base}async call(path,{method='GET',body,raw,csrf=true,origin,extraCookie=''}={}){const headers={'Content-Type':raw===undefined?'application/json':'application/octet-stream',Cookie:[this.cookie,extraCookie].filter(Boolean).join('; ')};if(csrf)headers['X-CSRF-Token']=this.csrf;if(method!=='GET')headers['Idempotency-Key']=randomBytes(16).toString('hex');if(origin)headers.Origin=origin;const response=await fetch(this.base+'/api'+path,{method,headers,body:raw??(body===undefined?undefined:JSON.stringify(body))});const c=response.headers.getSetCookie()[0];if(c)this.cookie=c.split(';')[0];const text=await response.text();let data;try{data=JSON.parse(text)}catch{data=text}return{response,data}}async login(name,password){const r=await this.call('/login',{method:'POST',body:{username:name,password}});assert.equal(r.response.status,200);this.csrf=r.data.csrfToken;return r.data.user}}
const route=(op,path='',extra={})=>'/drive/files/'+op+'?'+new URLSearchParams({root:'personal',path,...extra})
const admin=new Client(base);await admin.login('admin',password)
const guest=new Client(base);assert.equal((await guest.call('/drive/session')).response.status,401)
const session=await admin.call('/drive/session');assert.equal(session.response.status,200);assert.equal(session.data.user.username,'admin');assert.equal(session.data.csrfToken,admin.csrf)
assert.equal((await admin.call('/drive/login',{method:'POST',body:{}})).response.status,403)
assert.equal((await admin.call('/drive/logout',{method:'POST',body:{}})).response.status,403)
console.log('PASS Embedded gateway uses desktop session and cannot replace login/logout')
await mkdir('artifacts',{recursive:true});const username='drive_e2e_'+randomBytes(4).toString('hex'),pwd=randomBytes(20).toString('hex')
await createUser(base,admin,password,{username,password:pwd,displayName:'Embedded Files disposable test'})
const user=new Client(base),identity=await user.login(username,pwd);await writeFile('artifacts/drive-test-users.json',JSON.stringify([{id:identity.id,username}]))
const driveAdmin=new Client(drive);await driveAdmin.login('admin',password)
assert.equal((await user.call('/drive/files/list?root=system',{extraCookie:driveAdmin.cookie})).response.status,403)
assert.equal((await user.call(route('mkdir'),{method:'POST',body:{name:'Docs'},csrf:false})).response.status,403)
assert.equal((await user.call(route('mkdir'),{method:'POST',body:{name:'Docs'},origin:'https://evil.example'})).response.status,403)
console.log('PASS Drive-admin cookie cannot elevate embedded standard user; CSRF/Origin enforced')
assert.equal((await user.call(route('mkdir'),{method:'POST',body:{name:'Docs'}})).response.status,200)
const largeText='Gateway upload beyond old 128KB body cap\n'.repeat(5000)
assert.equal((await user.call(route('upload','Docs',{name:'a.txt'}),{method:'POST',raw:largeText})).response.status,201)
assert.equal((await user.call(route('copy','Docs/a.txt'),{method:'POST',body:{destination:'Docs/b.txt'}})).response.status,200)
const collision=await user.call(route('copy','Docs/a.txt'),{method:'POST',body:{destination:'Docs/b.txt'}});assert.equal(collision.response.status,409);assert.equal(collision.data.code,'CONFLICT')
assert.equal((await user.call(route('download','Docs/b.txt'))).data,largeText)
assert.equal((await admin.call(route('list','Docs'))).response.status,404)
console.log('PASS Streaming upload, copy bytes, collision protection and user isolation through desktop gateway')
await user.call('/logout',{method:'POST',body:{}})
assert.equal((await user.call(route('list'))).response.status,401)
console.log('PASS Desktop logout revokes embedded Files access')
