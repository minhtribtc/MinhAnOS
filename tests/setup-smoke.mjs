// First-run setup against a running fresh install: refuses a wrong setup code, creates the System Admin `admin` with
// the password in TEST_PASSWORD_FILE (which must differ from the setup code) and the first organization, then is
// closed. Run it first on a fresh install; the other smoke tests then sign in as that admin. Skips when already set up.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const base=process.env.TEST_URL||'http://127.0.0.1:8081'
const code=(await readFile(process.env.SETUP_CODE_FILE||'/run/bootstrap_password','utf8')).trim()
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/test_password','utf8')).trim()
const post=(path,body)=>fetch(base+path,{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify(body)})
const status=async()=>(await (await fetch(base+'/api/setup')).json()).required
if(!await status()){console.log('SKIP setup: this install is already set up');process.exit(0)}
assert.notEqual(password,code,'TEST_PASSWORD_FILE must differ from the setup code')
const input={setupCode:code,username:'admin',displayName:'System Admin',password,organizationName:'MinhAnOS'}
let r=await post('/api/setup',{...input,setupCode:'wrong-setup-code-000'})
assert.equal(r.status,403);assert.equal((await r.json()).code,'SETUP_CODE_INVALID')
r=await post('/api/setup',{...input,password:code})
assert.equal(r.status,400);assert.equal((await r.json()).code,'SETUP_WEAK_PASSWORD')
r=await fetch(base+'/api/setup',{method:'POST',headers:{'Content-Type':'application/json',Origin:'http://attacker.invalid'},body:JSON.stringify(input)})
assert.equal(r.status,403,'cross-origin setup')
console.log('PASS setup refuses a wrong code, a password equal to the code and another origin')
r=await post('/api/setup',input)
assert.equal(r.status,201,await r.text())
assert.equal(await status(),false)
r=await post('/api/setup',input)
assert.equal(r.status,409);assert.equal((await r.json()).code,'SETUP_DONE')
console.log('PASS setup creates the System Admin and the first organization once, then closes')
r=await post('/api/login',{username:'admin',password})
assert.equal(r.status,200)
console.log('PASS the new System Admin signs in')
