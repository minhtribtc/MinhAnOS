// Storage API contract: schema files = OpenAPI components, generated client up to date, runtime inventory.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import {generateStorageClient} from '../scripts/generate-identity-client.mjs'
const root=new URL('../',import.meta.url),json=async file=>JSON.parse(await readFile(new URL(file,root),'utf8'))
const index=await json('contracts/storage/index.v1.json'),api=await json('contracts/api/openapi.yaml'),runtime=await json('contracts/storage/status.v1.json')
const ajv=new Ajv2020({strict:true,allErrors:true});addFormats(ajv);const v={}
for(const entry of index.schemas){const schema=await json('contracts/storage/'+entry.file);v[entry.type]=ajv.compile(schema);const body=structuredClone(schema);delete body.$id;delete body.$schema;assert.deepEqual(api.components.schemas[entry.type],body,'OpenAPI storage drift: '+entry.type)}
const generated=await generateStorageClient()
assert.equal(await readFile(new URL('packages/api-client/src/storage.generated.js',root),'utf8'),generated.js,'storage client is stale: run npm run contracts:generate')
assert.equal(await readFile(new URL('packages/api-client/src/storage.generated.d.ts',root),'utf8'),generated.dts)
for(const [path,item] of Object.entries(api.paths))if(path.startsWith('/api/v1/storage/'))for(const [method,op] of Object.entries(item)){
 assert(runtime.implementedOperations.includes(op.operationId),'missing in status: '+op.operationId)
 const headers=(op.parameters||[]).filter(p=>p.in==='header').map(p=>p.name)
 if(method==='post')assert(headers.includes('Idempotency-Key'),op.operationId+' needs Idempotency-Key')
 if(method==='put')assert(headers.includes('If-Match'),op.operationId+' needs If-Match')
 if(method!=='get')assert(headers.includes('X-CSRF-Token'),op.operationId+' needs CSRF')
}
const usage={limitBytes:null,usedBytes:0,reservedBytes:0,measured:true}
assert(v.StorageHome({userId:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',username:'u',displayName:'U',share:'homes',version:1,usage}))
assert(!v.StorageHome({userId:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',username:'u',displayName:'U',share:'homes',version:1,usage,path:'/data/users/x'}),'no physical paths in DTOs')
assert(!v.StorageCreateShare({key:'Bad Key',name:'x',volume:'tank',limitBytes:null}))
assert(v.StorageCreateShare({key:'homes',name:'Homes',volume:'tank',limitBytes:null,hostsHomes:true}))
assert(v.StorageShareAccessInput({entries:[{subjectType:'group',subjectId:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',access:'read'}]}))
assert(!v.StorageShareAccessInput({entries:[{subjectType:'role',subjectId:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',access:'read'}]}),'only users and groups')
assert(!v.StorageShareAccessInput({entries:[{subjectType:'user',subjectId:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',access:'owner'}]}))
assert(v.StorageCreateQuotaFolder({path:'projects/2026',limitBytes:1073741824}))
for(const path of ['/abs','a//b','a/'])assert(!v.StorageCreateQuotaFolder({path,limitBytes:null}),'quota folder path '+path)
console.log('PASS Storage contracts: schemas = OpenAPI, generated client, If-Match/Idempotency-Key/CSRF on writes, no paths in DTOs')
