import assert from 'node:assert/strict'
import {readFile,writeFile,mkdir} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
import {createUser} from './identity-user.mjs'
const base=process.env.DRIVE_URL||'http://10.1.1.20:8082'
const platform=process.env.TEST_URL||'http://10.1.1.20:8081'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const results=[]
class Client{
 cookie='';csrf='';constructor(base){this.base=base}
 async call(path,{method='GET',body,raw,csrf=true,origin,key}={}){
  const headers={'Content-Type':raw===undefined?'application/json':'application/octet-stream',Cookie:this.cookie};if(csrf)headers['X-CSRF-Token']=this.csrf;if(origin)headers.Origin=origin;if(method!=='GET'&&!headers['Idempotency-Key'])headers['Idempotency-Key']=key??randomBytes(16).toString('hex')
  const response=await fetch(this.base+'/api'+path,{method,headers,body:raw??(body===undefined?undefined:JSON.stringify(body))});const cookie=response.headers.getSetCookie()[0];if(cookie)this.cookie=cookie.split(';')[0]
  const text=await response.text();let data;try{data=JSON.parse(text)}catch{data=text}return{response,data}
 }
 async login(name,password){const r=await this.call('/login',{method:'POST',body:{username:name,password}});assert.equal(r.response.status,200,JSON.stringify(r.data));this.csrf=r.data.csrfToken;return r.data.user}
}
const route=(op,root='personal',path='',extra={})=>'/files/'+op+'?'+new URLSearchParams({root,path,...extra})
const ok=name=>{console.log('PASS '+name);results.push(name)}
const guest=new Client(base);assert.equal((await guest.call(route('list'))).response.status,401);ok('Anonymous file access denied')
const platformAdmin=new Client(platform);await platformAdmin.login('admin',password)
const admin=new Client(base);await admin.login('admin',password);const adminRoots=await admin.call('/roots');assert.ok(adminRoots.data.roots.some(r=>r.id==='system'));ok('Independent Drive login and system root registry')
const fixture=[]
await mkdir('artifacts',{recursive:true})
for(let i=0;i<2;i++){const username='drive_e2e_'+randomBytes(4).toString('hex');const pwd=randomBytes(20).toString('hex');await createUser(platform,platformAdmin,password,{username,password:pwd,displayName:'Drive disposable test'});const client=new Client(base);const user=await client.login(username,pwd);fixture.push({username,id:user.id,client});await writeFile('artifacts/drive-test-users.json',JSON.stringify(fixture.map(({username,id})=>({username,id}))))}
const a=fixture[0].client,b=fixture[1].client
assert.equal((await a.call(route('list','system'))).response.status,403);assert.equal((await a.call(route('upload','system','srv',{name:'forbidden.txt'}),{method:'POST',raw:'blocked'})).response.status,403);ok('Standard user denied all host file access')
assert.equal((await a.call(route('mkdir'),{method:'POST',body:{name:'Documents'},csrf:false})).response.status,403)
assert.equal((await a.call(route('mkdir'),{method:'POST',body:{name:'Documents'},origin:'https://evil.example'})).response.status,403);ok('CSRF and cross-origin writes rejected')
const mkdirKey=randomBytes(16).toString('hex')
assert.equal((await a.call(route('mkdir'),{method:'POST',body:{name:'Documents'},key:mkdirKey})).response.status,200)
// A retried write with the same Idempotency-Key is replayed, not run twice; another request with that key is refused.
const replayed=await a.call(route('mkdir'),{method:'POST',body:{name:'Documents'},key:mkdirKey});assert.equal(replayed.response.status,200);assert.equal(replayed.response.headers.get('idempotent-replayed'),'true')
assert.equal((await a.call(route('mkdir'),{method:'POST',body:{name:'Other'},key:mkdirKey})).response.status,409)
assert.equal((await a.call(route('mkdir'),{method:'POST',body:{name:'NoKey'},key:''})).response.status,400);ok('Writes need an Idempotency-Key and are replayed')
assert.equal((await a.call(route('upload','personal','Documents',{name:'hello.txt'}),{method:'POST',raw:'Xin chào Drive — UTF-8'})).response.status,201)
const listed=await a.call(route('list','personal','Documents'));assert.equal(listed.data.entries[0].name,'hello.txt')
assert.equal((await b.call(route('list','personal','Documents'))).response.status,404)
assert.equal((await a.call(route('list','personal','../'+fixture[1].id))).response.status,400);ok('Real mkdir/upload/list and per-user storage isolation')
const conflict=await a.call(route('upload','personal','Documents',{name:'hello.txt'}),{method:'POST',raw:'overwrite'});assert.equal(conflict.response.status,409)
const download=await a.call(route('download','personal','Documents/hello.txt'));assert.equal(download.data,'Xin chào Drive — UTF-8');assert.match(download.response.headers.get('content-disposition'),/attachment/);ok('Download bytes preserved and duplicate upload cannot overwrite')
// Folder copy is a durable job (row 20): 202 + job, progress until succeeded, cancel of a finished job is a no-op.
assert.equal((await a.call(route('copy','personal','Documents'),{method:'POST',body:{destination:'Documents/inner'}})).response.status,400)
const queuedCopy=await a.call(route('copy','personal','Documents'),{method:'POST',body:{destination:'Documents copy'}});assert.equal(queuedCopy.response.status,202,JSON.stringify(queuedCopy.data));assert.equal(queuedCopy.data.job.type,'files.copy')
let copyJob=queuedCopy.data.job;for(let i=0;i<60&&['queued','running'].includes(copyJob.state);i++){await new Promise(r=>setTimeout(r,500));copyJob=(await a.call('/jobs/'+copyJob.id)).data}
assert.equal(copyJob.state,'succeeded',JSON.stringify(copyJob));assert.equal(copyJob.progress.unit,'bytes');assert.equal((await a.call(route('download','personal','Documents copy/hello.txt'))).data,'Xin chào Drive — UTF-8')
assert.equal((await a.call('/jobs/'+copyJob.id+'/cancel',{method:'POST',body:{}})).data.state,'succeeded');assert((await a.call('/jobs')).data.items.some(j=>j.id===copyJob.id))
assert.equal((await a.call('/jobs/00000000-0000-4000-8000-000000000000')).response.status,404);ok('Folder copy runs as a durable job with progress and idempotent cancel')
const text=await a.call(route('text','personal','Documents/hello.txt'));assert.equal(text.response.status,200)
assert.equal((await a.call(route('save-text','personal','Documents/hello.txt'),{method:'POST',body:{content:'bad',etag:'stale'}})).response.status,409)
const saved=await a.call(route('save-text','personal','Documents/hello.txt'),{method:'POST',body:{content:'Nội dung đã sửa',etag:text.data.etag}});assert.equal(saved.response.status,200);assert.match(saved.data.backup,/backup/)
assert.equal((await a.call(route('text','personal','Documents/hello.txt'))).data.content,'Nội dung đã sửa');ok('Text editor optimistic concurrency and backup')
assert.equal((await a.call(route('rename','personal','Documents/hello.txt'),{method:'POST',body:{name:'renamed.txt'}})).response.status,200)
assert.equal((await a.call(route('move','personal','Documents/renamed.txt'),{method:'POST',body:{destination:'moved.txt'}})).response.status,200)
const deleted=await a.call(route('trash','personal','moved.txt'),{method:'POST',body:{}});assert.equal(deleted.response.status,200)
assert.equal((await a.call(route('trash'))).data.length,1)
assert.equal((await b.call(route('restore'),{method:'POST',body:{id:deleted.data.item.id}})).response.status,404)
assert.equal((await a.call(route('restore'),{method:'POST',body:{id:deleted.data.item.id}})).response.status,200);ok('Rename/move/trash/restore and trash ownership isolation')
const systemDir='srv/minhanos-drive-smoke-'+randomBytes(5).toString('hex')
assert.equal((await admin.call(route('mkdir','system','srv'),{method:'POST',body:{name:systemDir.split('/').at(-1)}})).response.status,200)
try{
 assert.equal((await admin.call(route('upload','system',systemDir,{name:'test.conf'}),{method:'POST',raw:'original=true\n'})).response.status,201)
 const hostList=await admin.call(route('list','system',systemDir));assert.equal(hostList.data.entries[0].uid,0)
 const config=await admin.call(route('text','system',systemDir+'/test.conf'));assert.equal(config.data.content,'original=true\n')
 const changed=await admin.call(route('save-text','system',systemDir+'/test.conf'),{method:'POST',body:{content:'changed=true\n',etag:config.data.etag}});assert.equal(changed.response.status,200)
 const hostRead=await admin.call(route('text','system','etc/hostname'));assert.equal(hostRead.response.status,200)
 assert.equal((await admin.call(route('list','system','proc'))).response.status,403)
 assert.equal((await admin.call(route('remove','system',systemDir+'/test.conf'),{method:'POST',body:{}})).response.status,400)
 ok('Real Debian agent: root-owned file write/edit/backup, hostname read, virtual FS denied')
}finally{
 const list=await admin.call(route('list','system',systemDir));for(const entry of list.data.entries||[]){assert.equal((await admin.call(route('remove','system',entry.path),{method:'POST',body:{confirm:true}})).response.status,200)}
 assert.equal((await admin.call(route('remove','system',systemDir),{method:'POST',body:{confirm:true}})).response.status,200)
}
await a.call('/logout',{method:'POST',body:{}});assert.equal((await a.call(route('list'))).response.status,401);ok('Drive logout revokes independent session')
await writeFile('artifacts/drive-smoke.json',JSON.stringify({base,checks:results,date:new Date().toISOString()},null,2))
console.log(`Completed ${results.length} Drive checks. Exact disposable user IDs recorded for cleanup.`)
