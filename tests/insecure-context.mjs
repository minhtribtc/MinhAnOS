// MinhAnOS is commonly opened over plain HTTP on a LAN address (e.g. http://10.1.1.20:8081), which is not a
// secure context: crypto.randomUUID, navigator.clipboard and crypto.subtle are missing there. Frontend code must
// use createIdempotencyKey (or guard the API) so dialogs and writes keep working.
import assert from 'node:assert/strict'
import {readFile,readdir} from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import {createIdempotencyKey} from '../packages/api-client/src/index.js'
const root=new URL('../',import.meta.url)
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

const real=globalThis.crypto
Object.defineProperty(globalThis,'crypto',{value:{getRandomValues:a=>real.getRandomValues(a)},configurable:true})
try{
 const keys=new Set(Array.from({length:200},()=>createIdempotencyKey()))
 assert.equal(keys.size,200,'keys must be unique');for(const k of keys)assert.match(k,uuid)
}finally{Object.defineProperty(globalThis,'crypto',{value:real,configurable:true})}
assert.match(createIdempotencyKey(),uuid)

async function walk(url){let names;try{names=await readdir(url,{withFileTypes:true})}catch(e){if(e.code==='ENOENT')return [];throw e}const out=[];for(const n of names){if(['node_modules','dist'].includes(n.name))continue;const next=new URL(n.name+(n.isDirectory()?'/':''),url);if(n.isDirectory())out.push(...await walk(next));else out.push(next)}return out}
for(const dir of ['packages/','apps/'])for(const file of await walk(new URL(dir,root))){
 if(!/\.(vue|ts|js)$/.test(file.pathname)||file.pathname.endsWith('/packages/api-client/src/index.js'))continue
 const source=await readFile(file,'utf8')
 assert(!/crypto\.randomUUID\s*\(/.test(source),'crypto.randomUUID breaks on plain HTTP; use createIdempotencyKey: '+fileURLToPath(file))
}
console.log('PASS insecure context: idempotency keys without crypto.randomUUID, no secure-context-only UUID calls in frontend')
