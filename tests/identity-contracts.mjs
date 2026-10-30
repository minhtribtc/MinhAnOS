import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import {generateIdentityClient} from '../scripts/generate-identity-client.mjs'
import {createIdentityClient} from '../packages/api-client/src/identity.generated.js'
const root=new URL('../',import.meta.url),json=async file=>JSON.parse(await readFile(new URL(file,root),'utf8'))
const index=await json('contracts/identity/index.v1.json'),api=await json('contracts/api/openapi.yaml')
const deref=value=>Array.isArray(value)?value.map(deref):value&&typeof value==='object'?(value.$ref?deref(api.components.schemas[value.$ref.split('/').at(-1)]):Object.fromEntries(Object.entries(value).map(([k,v])=>[k,deref(v)]))):value
const ajv=new Ajv2020({strict:true,allErrors:true});addFormats(ajv);const validators={}
for(const entry of index.schemas){const schema=await json('contracts/identity/'+entry.file);validators[entry.type]=ajv.compile(schema);const body=structuredClone(schema);delete body.$id;delete body.$schema;assert.deepEqual(deref(api.components.schemas[entry.type]),body,'OpenAPI identity drift: '+entry.type)}
const id='68c8dedb-efbf-4de3-8a61-e7320b4d0d39'
const user={id,username:'example',displayName:'Example',status:'active',version:1}
assert(validators.IdentityUser(user));assert(!validators.IdentityUser({...user,id:'a'.repeat(64)}));assert(!validators.IdentityUser({...user,password_hash:'private'}));assert(!validators.IdentityUser({...user,role:'admin'}))
assert(!validators.IdentityPatchUser({role:'admin'}));assert(!validators.IdentityPatchUser({}));assert(validators.IdentityPatchUser({displayName:'Đổi tên'}))
assert(validators.IdentityCredentialRef({id,userId:id,kind:'password',state:'active',lastChangedAt:null}));assert(!validators.IdentityCredentialRef({id,userId:id,kind:'password',state:'active',lastChangedAt:null,secret:'private'}))
const session={id:'desktop:'+id,userId:id,realm:'desktop',current:true,createdAt:null,lastSeenAt:null,expiresAt:'2026-10-30T10:00:00Z',deviceHint:null}
assert(validators.IdentitySession(session));assert(!validators.IdentitySession({...session,tokenHash:'private'}));assert(!validators.IdentitySession({...session,id:'raw_bearer_secret'}))
const account=await json('contracts/manifests/accounts.app.json');assert.equal(account.id,'com.minhanos.account-center');assert.equal(account.sdk,'^0.1.0');assert(account.navigation.every(nav=>account.capabilities.includes(nav.requiredPermission)))
const runtime=await json('contracts/identity/status.v1.json');assert.equal(runtime.httpEndpointsEnabled,true);for(const methods of Object.values(api.paths))for(const op of Object.values(methods)){if(op?.operationId?.startsWith('identity'))assert.equal(op['x-implementation'],runtime.implementedOperations.includes(op.operationId)?'implemented':'planned')}
const generated=await generateIdentityClient();assert.equal(await readFile(new URL('packages/api-client/src/identity.generated.js',root),'utf8'),generated.js);assert.equal(await readFile(new URL('packages/api-client/src/identity.generated.d.ts',root),'utf8'),generated.dts)
const original=globalThis.fetch,seen=[]
try{globalThis.fetch=async(url,options)=>{seen.push({url,options});return {ok:true,json:async()=>user}}
const a=createIdentityClient(),b=createIdentityClient();a.setCSRF('a');b.setCSRF('b')
await a.identityGetUser({userId:'../other/user'});assert(seen[0].url.endsWith('/users/..%2Fother%2Fuser'))
await a.identityListUsers({search:'a & b',limit:25});assert.match(seen[1].url,/search=a\+%26\+b/)
await b.identityPatchUser({userId:id,ifMatch:'"v1"',body:{displayName:'Changed'}});assert.equal(seen[2].options.headers.get('X-CSRF-Token'),'b');assert.equal(seen[2].options.headers.get('If-Match'),'"v1"')
await a.identityCreateUser({idempotencyKey:'key-with-16-characters',body:{username:'next',displayName:'Next',password:'temporary-test-only'}});assert.equal(seen[3].options.headers.get('X-CSRF-Token'),'a');assert.equal(seen[3].options.headers.get('Idempotency-Key'),'key-with-16-characters')
await a.identitySetUserRole({userId:id,ifMatch:'"1"',idempotencyKey:'role-key-example',body:{role:'user-admin'}});assert.equal(seen[4].options.method,'PUT');assert.equal(seen[4].options.headers.get('If-Match'),'"1"')
assert.throws(()=>createIdentityClient('https://external.invalid'))
}finally{globalThis.fetch=original}
console.log('PASS Identity contracts: UUID DTOs, secret/mass-assignment rejection, planned manifest, schema/client drift, encoded paths and CSRF/precondition/idempotency headers')
