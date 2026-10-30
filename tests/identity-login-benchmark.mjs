// Small sequential VM comparison, not a capacity/security certification.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const password=(await readFile('/run/bootstrap_password','utf8')).trim()
const results=[]
for(const base of ['http://10.1.1.20:8081','http://10.1.1.20:8082']){
 const samples=[]
 for(let i=0;i<8;i++){
  const start=performance.now()
  const response=await fetch(base+'/api/login',{method:'POST',headers:{'Content-Type':'application/json',Origin:base},body:JSON.stringify({username:'admin',password})})
  assert.equal(response.status,200,'benchmark login unavailable')
  const session=await response.json();samples.push(performance.now()-start)
  const cookie=response.headers.getSetCookie()[0].split(';')[0]
  const out=await fetch(base+'/api/logout',{method:'POST',headers:{'Content-Type':'application/json',Origin:base,Cookie:cookie,'X-CSRF-Token':session.csrfToken},body:'{}'})
  assert.equal(out.status,200)
 }
 samples.sort((a,b)=>a-b)
 results.push({base,samples:8,p50ms:Math.round(samples[3]),p95ms:Math.round(samples[7])})
}
console.log(JSON.stringify({label:process.env.BENCHMARK_LABEL||'unspecified',results}))
