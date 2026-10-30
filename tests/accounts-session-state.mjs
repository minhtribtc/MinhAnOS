// Verify the native host cannot apply a late save from a previous account.
// No browser or live VM credentials: transport is deliberately isolated.
import assert from 'node:assert/strict'
import {mkdir} from 'node:fs/promises'
import {build} from 'esbuild'
import {createPinia,setActivePinia} from 'pinia'
await mkdir('artifacts',{recursive:true})
await build({entryPoints:['apps/desktop/src/store.ts'],outfile:'artifacts/accounts-store-test.mjs',bundle:true,platform:'node',format:'esm',external:['vue','pinia','@minhanos/api-client']})
const {useDesktop}=await import('../artifacts/accounts-store-test.mjs')
setActivePinia(createPinia())
const store=useDesktop(),original=globalThis.fetch,calls=[]
let releaseOld
try{
 globalThis.fetch=async(url,options)=>{
  calls.push({url,body:JSON.parse(options.body),csrf:options.headers.get('X-CSRF-Token')})
  if(calls.length===1)return new Promise(resolve=>{releaseOld=resolve})
  return {ok:true,json:async()=>({ok:true})}
 }
 store.user={id:'old-user',username:'old',displayName:'Old',role:'user'}
 store.preferences.notes='private draft from old account';store.saveSoon();const previous=store.persist()
 assert.equal(calls.length,1)
 store.expireSession()
 assert.equal(store.user,null);assert.equal(store.preferences.notes,'');assert.equal(store.preferences.windows.length,0)
 store.user={id:'new-user',username:'new',displayName:'New',role:'user'}
 store.preferences.notes='new account draft';store.saveSoon();await store.persist()
 releaseOld({ok:false,status:401,json:async()=>({code:'AUTH_REQUIRED',error:'old session'})})
 await previous
 assert.equal(store.user.username,'new');assert.equal(store.preferences.notes,'new account draft');assert.equal(store.saveError,'');assert.equal(store.saving,false)
 assert.equal(calls.length,2);assert.equal(calls[1].body.notes,'new account draft')
 store.expireSession()
}finally{globalThis.fetch=original}
console.log('PASS Account host session isolation: private drafts clear and late previous-account saves cannot alter the new session')
