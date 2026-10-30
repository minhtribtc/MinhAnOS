// Notification Center and Log Center API contracts: schema files = OpenAPI components, generated clients up to
// date, runtime inventory, CSRF/Idempotency-Key on writes, no prose in notices, bounded pages.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import {generateNotificationsClient,generateLogsClient} from '../scripts/generate-identity-client.mjs'
const root=new URL('../',import.meta.url),json=async file=>JSON.parse(await readFile(new URL(file,root),'utf8'))
const api=await json('contracts/api/openapi.yaml'),ajv=new Ajv2020({strict:true,allErrors:true});addFormats(ajv);const v={}
for(const [name,generate] of [['notifications',generateNotificationsClient],['logs',generateLogsClient]]){
 const index=await json(`contracts/${name}/index.v1.json`),runtime=await json(`contracts/${name}/status.v1.json`)
 for(const entry of index.schemas){const schema=await json(`contracts/${name}/`+entry.file);v[entry.type]=ajv.compile(schema);const body=structuredClone(schema);delete body.$id;delete body.$schema;assert.deepEqual(api.components.schemas[entry.type],body,`OpenAPI ${name} drift: `+entry.type)}
 const generated=await generate()
 assert.equal(await readFile(new URL(`packages/api-client/src/${name}.generated.js`,root),'utf8'),generated.js,name+' client is stale: run npm run contracts:generate')
 assert.equal(await readFile(new URL(`packages/api-client/src/${name}.generated.d.ts`,root),'utf8'),generated.dts)
 for(const [path,item] of Object.entries(api.paths))if(path.startsWith(`/api/v1/${name}/`))for(const [method,op] of Object.entries(item)){
  assert(runtime.implementedOperations.includes(op.operationId),'missing in status: '+op.operationId)
  const params=op.parameters||[],headers=params.filter(p=>p.in==='header').map(p=>p.name)
  if(method==='post')assert(headers.includes('Idempotency-Key')&&headers.includes('X-CSRF-Token'),op.operationId+' needs Idempotency-Key and CSRF')
  assert(!['put','patch','delete'].includes(method),op.operationId+': v1 exposes only reads and idempotent POST')
  const limit=params.find(p=>p.name==='limit');if(limit)assert(limit.schema.maximum<=100,op.operationId+' page must be bounded')
 }
}
const notice={id:'68c8dedb-efbf-4de3-8a61-e7320b4d0d39',source:'com.minhanos.account-center',kind:'login.failures',severity:'warning',mandatory:true,params:{count:10,realm:'desktop'},createdAt:'2026-10-31T00:00:00Z',readAt:null}
assert(v.NotificationItem(notice),JSON.stringify(v.NotificationItem.errors))
assert(!v.NotificationItem({...notice,title:'Sign-in failed'}),'notices carry no text')
assert(!v.NotificationItem({...notice,params:{detail:{nested:true}}}),'params are flat values')
assert(v.NotificationMarkRead({ids:[notice.id]})&&v.NotificationMarkRead({upTo:notice.createdAt}))
assert(!v.NotificationMarkRead({})&&!v.NotificationMarkRead({ids:[notice.id],upTo:notice.createdAt}),'exactly one of ids/upTo')
const event={id:notice.id,source:'identity',occurredAt:notice.createdAt,action:'identity.user.update',outcome:'denied',actor:{id:notice.id,username:'root'},target:notice.id,targetName:'root',requestId:'req_1',correlationId:null,details:{reasonCode:'LAST_ADMIN'}}
assert(v.LogEvent(event),JSON.stringify(v.LogEvent.errors))
assert(!v.LogEvent({...event,source:'drive'}),'Files audit (personal paths) is not a Log Center source')
assert(v.LogSourceList({items:[{id:'identity',appId:'com.minhanos.account-center',retentionDays:365},{id:'storage',appId:'com.minhanos.storage',retentionDays:null}]}))
console.log('PASS notifications/logs contracts: schemas = OpenAPI, generated clients, CSRF/Idempotency-Key on writes, bounded pages, no prose in notices')
