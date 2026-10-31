import {readFile,writeFile,mkdir} from 'node:fs/promises'
import {compile} from 'json-schema-to-typescript'
const root=new URL('../',import.meta.url)
const sources=[['manifests/app-manifest.v1.schema.json','AppManifest'],['api/error.v1.schema.json','ApiErrorEnvelope'],['api/error.legacy.schema.json','LegacyErrorEnvelope'],['api/profile.schema.json','AccountProfile'],['files/file-resource.v1.schema.json','FileResource'],['files/acl.v1.schema.json','FileAcl'],['jobs/job.v1.schema.json','Job'],['uploads/upload.v1.schema.json','UploadSession'],['authz/check.v1.schema.json','AuthorizationCheck'],['events/event.v1.schema.json','PlatformEvent']]
const identityIndex=JSON.parse(await readFile(new URL('contracts/identity/index.v1.json',root),'utf8'))
sources.push(...identityIndex.schemas.map(entry=>['identity/'+entry.file,entry.type]))
const storageIndex=JSON.parse(await readFile(new URL('contracts/storage/index.v1.json',root),'utf8'))
sources.push(...storageIndex.schemas.map(entry=>['storage/'+entry.file,entry.type]))
for(const module of ['notifications','logs','files-api','desktop']){const index=JSON.parse(await readFile(new URL(`contracts/${module}/index.v1.json`,root),'utf8'));sources.push(...index.schemas.map(entry=>[module+'/'+entry.file,entry.type]))}
export async function generate(){let output='// Generated from contracts/*.schema.json. Run npm run contracts:generate. Do not edit.\n';for(const [path,name] of sources){const schema=JSON.parse(await readFile(new URL('contracts/'+path,root),'utf8'));delete schema.$id;schema.title=name;output+=await compile(schema,name,{bannerComment:'',additionalProperties:false})+'\n'}return output.trimEnd()+'\n'}
if(process.argv[1]===new URL(import.meta.url).pathname || process.argv[1]?.replaceAll('\\','/').endsWith('/scripts/generate-contract-types.mjs')){await mkdir(new URL('packages/shared-types/src/',root),{recursive:true});await writeFile(new URL('packages/shared-types/src/index.d.ts',root),await generate())}
