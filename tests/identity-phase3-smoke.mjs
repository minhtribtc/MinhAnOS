import assert from 'node:assert/strict'
import {readFile,writeFile,mkdir} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
const ajv=new Ajv2020({strict:true});addFormats(ajv)
const validators={}
for(const [name,file] of Object.entries({user:'user',users:'user-page',roles:'role-page',permissions:'permission-page',audit:'audit-page'}))validators[name]=ajv.compile(JSON.parse(await readFile(`contracts/identity/${file}.v1.schema.json`,'utf8')))
const error=ajv.compile(JSON.parse(await readFile('contracts/api/error.v1.schema.json','utf8')))
const password=(await readFile('/run/bootstrap_password','utf8')).trim()
class Client{
 cookie='';csrf=''
 constructor(base='http://10.1.1.20:8081'){this.base=base}
 async call(path,method='GET',body,headers={}){
  const response=await fetch(this.base+path,{method,headers:{'Content-Type':'application/json',Origin:this.base,Cookie:this.cookie,'X-CSRF-Token':this.csrf,...headers},body:body===undefined?undefined:JSON.stringify(body)})
  const data=await response.json();if(path.startsWith('/api/v1/')&&response.status>=400)assert(error(data),JSON.stringify(error.errors))
  const cookie=response.headers.getSetCookie()[0];if(cookie)this.cookie=cookie.split(';')[0]
  return {status:response.status,data,etag:response.headers.get('ETag')}
 }
 async login(name,password){const r=await this.call('/api/login','POST',{username:name,password});assert.equal(r.status,200);this.csrf=r.data.csrfToken}
 async fresh(password){assert.equal((await this.call('/api/v1/identity/reauthenticate','POST',{password})).status,200)}
}
const prefix='/api/v1/identity',admin=new Client();await admin.login('admin',password);await admin.fresh(password)
const me=await admin.call(prefix+'/me');assert.equal(me.status,200);assert(validators.user(me.data));assert(me.etag)
const fixtures=[];await mkdir('artifacts',{recursive:true})
const temporary=randomBytes(24).toString('hex')
async function create(client){
 const body={username:'identity_e2e_'+randomBytes(4).toString('hex'),displayName:'Identity disposable test',password:temporary},key=randomBytes(16).toString('hex')
 const r=await client.call(prefix+'/users','POST',body,{'Idempotency-Key':key});assert.equal(r.status,201);assert(validators.user(r.data))
 fixtures.push({id:r.data.id,username:r.data.username});await writeFile('artifacts/identity-phase3-users.json',JSON.stringify(fixtures))
 const replay=await client.call(prefix+'/users','POST',body,{'Idempotency-Key':key});assert.equal(replay.status,201);assert.deepEqual(replay.data,r.data)
 return r
}
async function role(user,value){const r=await admin.call(prefix+`/users/${user.data.id}/role`,'PUT',{role:value},{'If-Match':user.etag,'Idempotency-Key':randomBytes(16).toString('hex')});assert.equal(r.status,200);return r}
const web=await role(await create(admin),'system-admin'),delegate=await role(await create(admin),'user-admin')
const webDrive=new Client('http://10.1.1.20:8082');await webDrive.login(web.data.username,temporary)
assert.equal((await webDrive.call('/api/files/list?root=system')).status,403)
const webDesktop=new Client();await webDesktop.login(web.data.username,temporary);assert.equal((await webDesktop.call('/api/drive/files/list?root=system')).status,403);await webDesktop.call('/api/logout','POST',{});
console.log('PASS new System Admin cannot use host filesystem operator privileges in either realm')
const userAdmin=new Client();await userAdmin.login(delegate.data.username,temporary);await userAdmin.fresh(temporary)
assert.equal((await userAdmin.call(prefix+`/users/${delegate.data.id}/role`,'PUT',{role:'system-admin'},{'If-Match':delegate.etag,'Idempotency-Key':randomBytes(16).toString('hex')})).status,403)
assert.equal((await userAdmin.call(prefix+`/users/${web.data.id}/disable`,'POST',{}, {'If-Match':web.etag,'Idempotency-Key':randomBytes(16).toString('hex')})).status,403)
let plain=await create(userAdmin)
const stale=await admin.call(prefix+`/users/${plain.data.id}`,'PATCH',{displayName:'Changed'},{'If-Match':'"999"'});assert.equal(stale.status,412)
for(const action of ['disable','enable','disable']){plain=await userAdmin.call(prefix+`/users/${plain.data.id}/${action}`,'POST',{}, {'If-Match':plain.etag,'Idempotency-Key':randomBytes(16).toString('hex')});assert.equal(plain.status,200);assert(validators.user(plain.data))}
const removed=await userAdmin.call(prefix+`/users/${plain.data.id}`,'DELETE',undefined,{'If-Match':plain.etag,'Idempotency-Key':randomBytes(16).toString('hex')});assert.equal(removed.status,200)
for(const [path,type] of [['users?limit=1','users'],['roles','roles'],['permissions','permissions'],['audit-events?limit=5','audit']]){
 const r=await admin.call(prefix+'/'+path);assert.equal(r.status,200);assert(validators[type](r.data),JSON.stringify(validators[type].errors))
 if(r.data.nextCursor){const next=await admin.call(prefix+'/'+path+'&cursor='+encodeURIComponent(r.data.nextCursor));assert.equal(next.status,200)}
}
await admin.call('/api/logout','POST',{});await userAdmin.call('/api/logout','POST',{});await webDrive.call('/api/logout','POST',{})
console.log('PASS Identity v1 DTO schemas, delegated ceiling, replay, ETag, cursor pages, status transitions, soft-delete and session cleanup; exact fixture IDs recorded')
