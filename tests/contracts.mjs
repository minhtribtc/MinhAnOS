import {assertAppCatalogCompatibility} from './app-catalog-compatibility.mjs'
import {validateAppCatalog} from '../scripts/validate-app-catalog.mjs'
import assert from 'node:assert/strict'
import {readFile,readdir,stat} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import YAML from 'yaml'
import semver from 'semver'
import {generate} from '../scripts/generate-contract-types.mjs'
import {ApiError,createApiClient} from '../packages/api-client/src/index.js'
const root=new URL('../',import.meta.url)
const json=async path=>JSON.parse(await readFile(new URL(path,root),'utf8'))
const ajv=new Ajv2020({strict:true,allErrors:true});addFormats(ajv)
const schemaPaths=['manifests/app-manifest.v1','api/error.v1','api/error.legacy','api/profile','files/file-resource.v1','files/acl.v1','jobs/job.v1','uploads/upload.v1','authz/check.v1','events/event.v1']
const validators={};for(const path of schemaPaths){const schema=await json('contracts/'+path+'.schema.json');validators[path]=ajv.compile(schema)}
const failure={code:'JOB_INTERRUPTED',message:'Worker interrupted',requestId:'req_test',details:{}}
const job={id:'job_1',type:'files.copy',state:'queued',progress:{completed:0,total:10,unit:'items'},attempt:0,maxAttempts:3,createdAt:'2026-10-30T00:00:00Z',updatedAt:'2026-10-30T00:00:00Z',error:null}
assert(validators['jobs/job.v1'](job));assert(validators['jobs/job.v1']({...job,state:'failed',error:failure}));assert(!validators['jobs/job.v1']({...job,state:'paused'}))
const upload={id:'upload_1',state:'uploading',size:10,partSize:5,receivedParts:[1],checksum:'a'.repeat(64),expiresAt:'2026-10-31T00:00:00Z',fileId:null}
assert(validators['uploads/upload.v1'](upload));assert(!validators['uploads/upload.v1']({...upload,receivedParts:[1,1]}));assert(!validators['uploads/upload.v1']({...upload,checksum:'sha1'}))
const acl={fileId:'file_1',version:1,etag:'etag_1',ownerId:'user_1',inherit:false,entries:[{principal:'user:user_1',allow:['read','share'],deny:[]}]}
assert(validators['files/acl.v1'](acl));assert(!validators['files/acl.v1']({...acl,entries:[{principal:'root',allow:['all'],deny:[]}]}))
const manifests=await readdir(new URL('contracts/manifests/',root));const permissions=YAML.parse(await readFile(new URL('contracts/authz/permissions.yaml',root),'utf8')).permissions
const sdk=(await json('packages/app-sdk/package.json')).version
for(const file of manifests.filter(f=>f.endsWith('.app.json'))){const manifest=await json('contracts/manifests/'+file);assert(validators['manifests/app-manifest.v1'](manifest),JSON.stringify(validators['manifests/app-manifest.v1'].errors));assert.equal(semver.valid(manifest.version),manifest.version);if(manifest.id==='com.minhanos.files')assert.equal(manifest.version,(await json('packages/files-app/package.json')).version);assert(semver.validRange(manifest.sdk));assert(semver.satisfies(sdk,manifest.sdk),'SDK range incompatible: '+file);assert(manifest.capabilities.every(c=>c in permissions),'Unknown capability')}
const catalog=validateAppCatalog(await json('contracts/apps/catalog.v1.json'))
for(const app of catalog.apps)for(const path of app.currentPaths)await stat(new URL(path,root))
assert.equal((await json('apps/desktop/package.json')).name,catalog.apps.find(a=>a.key==='desktop').moduleName)
assert.equal((await json('apps/drive/package.json')).name,catalog.apps.find(a=>a.key==='files').moduleName)
const renamed=structuredClone(catalog);renamed.apps[1].displayName='Files';assertAppCatalogCompatibility(catalog,renamed)
const removedId=structuredClone(catalog);removedId.apps[1].appId='com.minhanos.other-files';assert.throws(()=>assertAppCatalogCompatibility(catalog,removedId))
const removedAlias=structuredClone(catalog);removedAlias.apps[1].legacyRegistryIds=[];assert.throws(()=>assertAppCatalogCompatibility(catalog,removedAlias))
const duplicates=structuredClone(catalog);duplicates.apps[1].appId=duplicates.apps[0].appId;assert.throws(()=>validateAppCatalog(duplicates))
const aliases=structuredClone(catalog);aliases.apps[2].legacyRegistryIds=['files'];assert.throws(()=>validateAppCatalog(aliases))
const paths=structuredClone(catalog);paths.apps[0].currentPaths=['apps/../secrets'];assert.throws(()=>validateAppCatalog(paths))
const moduleNames=structuredClone(catalog);moduleNames.apps[1].moduleName=moduleNames.apps[0].moduleName;assert.throws(()=>validateAppCatalog(moduleNames))
for(const file of manifests.filter(f=>f.endsWith('.app.json'))){const manifest=await json('contracts/manifests/'+file);assert(catalog.apps.some(a=>a.appId===manifest.id),'Manifest ID missing from app catalog')}
const manifest=await json('contracts/manifests/files.app.json');assert(!validators['manifests/app-manifest.v1']({...manifest,id:'files'}));assert(!validators['manifests/app-manifest.v1']({...manifest,grantedCapabilities:['files.read']}));assert(!semver.satisfies(sdk,'^1.0.0'))
assert(validators['authz/check.v1']({action:'files.read',resource:'file:123',context:{appId:'com.minhanos.files'}}));assert(!validators['authz/check.v1']({subject:'user:admin',action:'files.read',resource:'file:123',context:{appId:'com.minhanos.files'}}))
for(const [path,fixture] of [['api/error.legacy','error.legacy'],['api/error.v1','error.v1']]){const body=await json('contracts/fixtures/'+fixture+'.json');assert(validators[path](body));const error=new ApiError(403,body);assert.equal(error.code,path.endsWith('legacy')?'ACCESS_DENIED':'FILES_PERMISSION_DENIED');if(path.endsWith('v1'))assert.equal(error.requestId,'req_contract_test')}
assert.equal(new ApiError(500,null).code,'REQUEST_FAILED')
const original=globalThis.fetch
try{for(const fixture of ['error.legacy','error.v1']){const body=await json('contracts/fixtures/'+fixture+'.json');globalThis.fetch=async()=>({ok:false,status:403,json:async()=>body});await assert.rejects(createApiClient().request('/test'),e=>e instanceof ApiError&&e.code===new ApiError(403,body).code)}}finally{globalThis.fetch=original}
const event={eventId:'event_test',name:'files.updated.v1',schemaVersion:1,occurredAt:'2026-10-30T00:00:00Z',source:'service:files',subject:'file:123',correlationId:'req_test',data:{fileId:'123',version:2,etag:'etag_2'}};assert(validators['events/event.v1'](event),JSON.stringify(validators['events/event.v1'].errors));assert(!validators['events/event.v1']({...event,data:{}}))
for(const name of ['jobs/jobs.v1','uploads/uploads.v1','events/events.v1','files/file-resource.v1','manifests/lifecycle.v1','migrations/modules.v1']){const doc=YAML.parse(await readFile(new URL('contracts/'+name+'.yaml',root),'utf8'));assert.equal(doc.contractVersion,'1');assert.equal(doc.implementation,'planned')}
const generated=await generate();assert.equal(await readFile(new URL('packages/shared-types/src/index.d.ts',root),'utf8'),generated,'Stale generated shared-types: npm run contracts:generate')
const api=YAML.parse(await readFile(new URL('contracts/api/openapi.yaml',root),'utf8'));assert.equal(api.openapi,'3.1.0');for(const [path,name] of [['api/profile','Profile'],['api/error.legacy','LegacyError'],['api/error.v1','Error'],['files/file-resource.v1','FileResource'],['files/acl.v1','FileAcl'],['jobs/job.v1','Job'],['uploads/upload.v1','Upload'],['authz/check.v1','AuthorizationCheck']]){const schema=await json('contracts/'+path+'.schema.json');delete schema.$id;delete schema.$schema;assert.deepEqual(api.components.schemas[name],schema,'OpenAPI/schema drift: '+name)}
const identityRuntime=JSON.parse(await readFile(new URL('contracts/identity/status.v1.json',root),'utf8'));
for(const [path,item] of Object.entries(api.paths))for(const [method,op] of Object.entries(item)){if(!['get','post','put','patch','delete'].includes(method))continue;assert.equal(op['x-implementation'],path==='/api/profile'||(path.startsWith('/api/v1/identity/')&&identityRuntime.implementedOperations.includes(op.operationId))?'implemented':'planned')}
console.log('PASS contracts: app naming/alias catalog, JSON Schema, event payloads, authz subject rejection, manifest permissions/SDK compatibility, generated types, legacy/v1 client errors and planned API status')
