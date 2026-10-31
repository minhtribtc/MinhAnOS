// Files and Desktop API contracts (app standard row 15): schema files = OpenAPI components, generated clients up to
// date, CSRF + Idempotency-Key on Files writes, If-Match on versioned writes, paged listings, and no route in the
// Go code that OpenAPI does not document.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import {generateFilesClient,generateDesktopClient} from '../scripts/generate-identity-client.mjs'
const root=new URL('../',import.meta.url),text=file=>readFile(new URL(file,root),'utf8'),json=async file=>JSON.parse(await text(file))
const api=await json('contracts/api/openapi.yaml')
const ajv=new Ajv2020({strict:true,allErrors:true});addFormats(ajv);const v={}
for(const [dir,module,prefix] of [['files-api','files','/api/drive/'],['desktop','desktop','/api/']]){
 const index=await json(`contracts/${dir}/index.v1.json`),runtime=await json(`contracts/${dir}/status.v1.json`)
 for(const entry of index.schemas){const schema=await json(`contracts/${dir}/${entry.file}`);v[entry.type]=ajv.compile(schema);const body=structuredClone(schema);delete body.$id;delete body.$schema;assert.deepEqual(api.components.schemas[entry.type],body,entry.type+' differs from OpenAPI')}
 const documented=[]
 for(const [path,item] of Object.entries(api.paths))for(const [method,op] of Object.entries(item)){
  if(op['x-module']!==module)continue
  documented.push(op.operationId)
  assert(path.startsWith(prefix),op.operationId+' outside '+prefix)
  assert.equal(op['x-implementation'],'implemented')
  assert(runtime.implementedOperations.includes(op.operationId),'missing in status: '+op.operationId)
  assert(op.responses.default?.content?.['application/json']?.schema?.$ref==='#/components/schemas/LegacyError',op.operationId+' needs the error envelope')
  const headers=(op.parameters||[]).filter(p=>p.in==='header').map(p=>p.name)
  // Pre-session writes: login, first-run setup and public links (Origin check plus credentials, the setup code or
  // the link token).
  const open=['filesLogin','desktopLogin','desktopSetup'].includes(op.operationId)||op.operationId.startsWith('filesPublic')
  if(method!=='get'&&!open)assert(headers.includes('X-CSRF-Token'),op.operationId+' needs CSRF')
  if(module==='files'&&path.startsWith('/api/drive/files/')&&method==='post')assert(headers.includes('Idempotency-Key'),op.operationId+' needs Idempotency-Key')
  if(method==='put'||op.operationId==='filesSaveText')assert(headers.includes('If-Match'),op.operationId+' needs If-Match')
 }
 assert.deepEqual([...documented].sort(),[...runtime.implementedOperations].sort(),dir+' status lists undocumented operations')
}
const list=api.paths['/api/drive/files/list'].get
assert(list.parameters.some(p=>p.name==='limit'&&p.schema.maximum===1000)&&list.parameters.some(p=>p.name==='cursor'),'listing is paged')
for(const [name,generate] of [['files',generateFilesClient],['desktop',generateDesktopClient]]){
 const generated=await generate()
 assert.equal(await text(`packages/api-client/src/${name}.generated.js`),generated.js,name+' client is stale: run npm run contracts:generate')
 assert.equal(await text(`packages/api-client/src/${name}.generated.d.ts`),generated.dts)
}
// Every route in the Go code is documented (no undocumented endpoints).
const drive=await text('internal/drive/server.go'),platform=await text('internal/platform/server.go')
const driveRoutes=[...drive.matchAll(/mux\.HandleFunc\("(GET|POST) \/api\/([a-z-]+)"/g)].map(([,m,p])=>[m,p])
 .concat([...drive.matchAll(/r\.URL\.Path == "\/api\/([a-z-]+)" && r\.Method == "(GET|POST)"/g)].map(([,p,m])=>[m,p]))
 .concat([...drive.slice(drive.indexOf('func ServeEngine')).matchAll(/case "([a-z-]+)":/g)].map(([,p])=>[['list','download','text','search','recent','archive'].includes(p)?'GET':'POST','files/'+p]))
for(const [method,route] of driveRoutes){if(route==='profile')continue;const item=api.paths['/api/drive/'+route];assert(item?.[method.toLowerCase()]||(route==='files/trash'&&item?.get&&item?.post),'undocumented Files route '+method+' /api/'+route)}
assert(api.paths['/api/drive/files/upload']?.post,'undocumented upload')
// Durable jobs (row 20): the Go job routes are documented, copy may answer 202, and a Files job is a job.v1.
const jobs=await text('internal/drive/jobs.go')
assert(drive.includes('r.URL.Path == "/api/jobs"')&&jobs.includes('parts[1] == "cancel" && r.Method == "POST"'),'job routes moved: update this check')
for(const [route,method] of [['jobs','get'],['jobs/{id}','get'],['jobs/{id}/cancel','post']])assert(api.paths['/api/drive/'+route]?.[method],'undocumented Files route '+method+' '+route)
assert(api.paths['/api/drive/files/copy'].post.responses['202'],'copy documents the queued job')
const sharedJob=ajv.compile(await json('contracts/jobs/job.v1.schema.json'))
const sample={id:'0a0b0c0d-0000-4000-8000-000000000001',type:'files.copy',state:'running',progress:{completed:1,total:2,unit:'bytes'},attempt:1,maxAttempts:3,createdAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:00:01Z',error:null}
assert(v.FilesJob(sample)&&sharedJob(sample),'Files job is a job.v1')
assert(v.FilesWriteResult({ok:true,job:sample}),'202 body carries the job')
assert(!v.FilesJob({...sample,type:'files.move'}),'only copy jobs')
for(const [,method,route] of platform.matchAll(/mux\.Handle(?:Func)?\("(GET|POST|PUT|DELETE) (\/api\/[a-z-]+)"/g))assert(api.paths[route]?.[method.toLowerCase()],'undocumented Desktop route '+method+' '+route)
// DTO rules: no unknown fields, preferences bounded, roots carry names only for Shared Folders.
assert(v.FilesEntry({name:'a.txt',path:'a.txt',kind:'file',size:1,modified:'2026-10-31T00:00:00Z',mode:'-rw-r-----',uid:10001,gid:10001,etag:'x'}))
assert(!v.FilesEntry({name:'a.txt',path:'a.txt',kind:'file',size:1,modified:'2026-10-31T00:00:00Z',mode:'',uid:0,gid:0,etag:'x',owner:'root'}),'unknown fields rejected')
assert(v.FilesRootList({roots:[{id:'personal',kind:'home',access:'write'},{id:'share:team',kind:'share',name:'Team',access:'read'}],homes:true,maxUploadBytes:1,maxTextBytes:1}))
assert(!v.FilesRootList({roots:[{id:'../x',kind:'share',access:'read'}],homes:true,maxUploadBytes:1,maxTextBytes:1}))
assert(!v.FilesRemoveInput({confirm:false}),'permanent delete needs confirm')
assert(!v.DesktopPreferences({theme:'light',wallpaper:'sage',mode:'focus',windows:[],notes:'x'.repeat(32001),pinned:[]}),'notes bounded')
assert(!v.DesktopSystemStatus({version:'1',hostname:'h',os:'linux',architecture:'amd64',goVersion:'go',cpus:1,goroutines:1,heapBytes:1,uptimeSeconds:1,diskTotalBytes:1,diskFreeBytes:1,database:'connected',integrations:[]}),'no placeholder integrations')
console.log('PASS Files/Desktop contracts: schemas = OpenAPI, generated clients, CSRF/Idempotency-Key/If-Match on writes, paged listing, every Go route documented')
