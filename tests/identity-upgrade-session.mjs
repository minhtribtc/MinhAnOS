// Explicit pre/post deployment check. Private cookie file must never be committed.
import assert from 'node:assert/strict'
import {readFile,writeFile,unlink,stat} from 'node:fs/promises'
const file=process.env.TEST_SESSION_FILE
assert(file,'private TEST_SESSION_FILE required')
const mode=process.env.TEST_SESSION_MODE
if(mode==='capture'){
 const password=(await readFile('/run/bootstrap_password','utf8')).trim()
 const sessions=[]
 for(const base of ['http://10.1.1.20:8081','http://10.1.1.20:8082']){
  const r=await fetch(base+'/api/login',{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify({username:'admin',password})})
  assert.equal(r.status,200);const data=await r.json()
  sessions.push({base,cookie:r.headers.getSetCookie()[0].split(';')[0],csrf:data.csrfToken})
 }
 await writeFile(file,JSON.stringify(sessions),{mode:0o600,flag:'wx'})
 console.log('PASS existing sessions saved privately before upgrade')
}else if(mode==='verify'){
 assert.equal((await stat(file)).mode&0o077,0,'session file must be private')
 const sessions=JSON.parse(await readFile(file,'utf8'))
 for(const {base,cookie,csrf} of sessions){
  assert.equal((await fetch(base+'/api/session',{headers:{Cookie:cookie}})).status,200,'pre-upgrade session lost')
  if(base.endsWith(':8081'))assert.equal((await fetch(base+'/api/drive/session',{headers:{Cookie:cookie}})).status,200)
  assert.equal((await fetch(base+'/api/logout',{method:'POST',headers:{'Content-Type':'application/json',Origin:base,Cookie:cookie,'X-CSRF-Token':csrf},body:'{}'})).status,200)
 }
 await unlink(file)
 console.log('PASS both pre-upgrade cookie realms and Files gateway; exact sessions logged out and private file removed')
}else throw new Error('TEST_SESSION_MODE must be capture or verify')
