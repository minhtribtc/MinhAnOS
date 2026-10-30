// Run only against the disposable standalone installation on loopback port 8083.
import assert from 'node:assert/strict'
import {readFile,writeFile,unlink} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
const base='http://127.0.0.1:8083'
const directory=process.env.TEST_FRESH_PRIVATE_DIR
assert(directory,'private fixture directory required')
const original=(await readFile('/run/bootstrap_password','utf8')).trim()
const login=password=>fetch(base+'/api/login',{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify({username:'admin',password})})
if(process.env.TEST_FRESH_STAGE==='capture'){
 const r=await login(original);assert.equal(r.status,200)
 await writeFile(directory+'/fresh-session.json',JSON.stringify({cookie:r.headers.getSetCookie()[0].split(';')[0]}),{mode:0o600,flag:'wx'})
 await writeFile(directory+'/fresh-password',randomBytes(32).toString('hex')+'\n',{mode:0o600,flag:'wx'})
 console.log('PASS fresh standalone login; private recovery fixtures created')
}else if(process.env.TEST_FRESH_STAGE==='verify'){
 const {cookie}=JSON.parse(await readFile(directory+'/fresh-session.json','utf8'))
 assert.equal((await fetch(base+'/api/session',{headers:{Cookie:cookie}})).status,401)
 assert.equal((await login(original)).status,401)
 const r=await login((await readFile(directory+'/fresh-password','utf8')).trim());assert.equal(r.status,200)
 const session=await r.json()
 assert.equal((await fetch(base+'/api/logout',{method:'POST',headers:{'Content-Type':'application/json',Origin:base,Cookie:r.headers.getSetCookie()[0].split(';')[0],'X-CSRF-Token':session.csrfToken},body:'{}'})).status,200)
 await unlink(directory+'/fresh-session.json');await unlink(directory+'/fresh-password')
 console.log('PASS real local recovery: old session/password denied, new password accepted, exact private fixtures removed')
}else throw new Error('capture or verify stage required')
