import {readFile,writeFile,mkdir} from 'node:fs/promises'
const root=new URL('../',import.meta.url)
export async function generateIdentityClient(){
 const api=JSON.parse(await readFile(new URL('contracts/api/openapi.yaml',root),'utf8'))
 const types=new Set(),methods=[],implementations=[]
 const typeOf=schema=>{const name=schema?.$ref?.split('/').at(-1);if(!name||!api.components.schemas[name])throw new Error('Identity client requires a named schema');types.add(name);return name}
 for(const [path,item] of Object.entries(api.paths)){
  if(!path.startsWith('/api/v1/identity/'))continue
  for(const [method,op] of Object.entries(item)){
   if(!['get','post','put','patch','delete'].includes(method))continue
   const params=[...(item.parameters||[]),...(op.parameters||[])],fields=[],code=[]
   let url=JSON.stringify(path.slice('/api/v1/identity'.length))
   for(const param of params.filter(p=>p.in==='path')){fields.push(`${param.name}:string`);code.push(`path=path.replace(${JSON.stringify('{'+param.name+'}')},encodeURIComponent(args.${param.name}));`)}
   code.unshift(`let path=${url};`)
   const queries=params.filter(p=>p.in==='query')
   if(queries.length){code.push('const query=new URLSearchParams();');for(const param of queries){fields.push(`${param.name}?:${param.schema.type==='integer'?'number':'string'}`);code.push(`if(args.${param.name}!==undefined)query.set(${JSON.stringify(param.name)},String(args.${param.name}));`)}code.push("if(query.size)path+='?'+query;")}
   const headers=[]
   for(const [wire,name] of [['If-Match','ifMatch'],['Idempotency-Key','idempotencyKey']])if(params.some(p=>p.in==='header'&&p.name===wire)){fields.push(`${name}:string`);headers.push(`${JSON.stringify(wire)}:args.${name}`)}
   const request=op.requestBody?.content?.['application/json']?.schema
   if(request)fields.push('body:'+typeOf(request))
   const success=Object.entries(op.responses).find(([status])=>/^2[0-9][0-9]$/.test(status))?.[1];if(!success)throw new Error('Identity operation missing success response');const response=typeOf(success.content['application/json'].schema)
   const required=fields.some(field=>!field.split(':')[0].endsWith('?'));methods.push(`${op.operationId}(${fields.length?'args'+(required?'':'?')+':{'+fields.join(';')+'}':''}):Promise<${response}>`)
   code.push(`return client.request(path,{method:${JSON.stringify(method.toUpperCase())},headers:{${headers.join(',')}},${request?'body:JSON.stringify(args.body)':''}});`)
   implementations.push(`${op.operationId}(args={}){${code.join('')}}`)
  }
 }
 const js=["// Generated from OpenAPI Identity operations. Do not edit.","import {createApiClient} from './index.js'",`export function createIdentityClient(base='/api/v1/identity'){if(!base.startsWith('/')||base.startsWith('//')||base.includes('://'))throw new Error('Identity base must be a same-origin path');const client=createApiClient(base);return {setCSRF:client.setCSRF,${implementations.join(',')}}}`,''].join('\n')
 const dts=["// Generated from OpenAPI Identity operations. Do not edit.",`import type {${[...types].sort().join(',')}} from '@minhanos/shared-types'`,`export function createIdentityClient(base?:string):{setCSRF(value:string):void;${methods.join(';')}}`,''].join('\n')
 return {js,dts}
}
if(process.argv[1]?.replaceAll('\\','/').endsWith('/generate-identity-client.mjs')){const output=await generateIdentityClient();await mkdir(new URL('packages/api-client/src/',root),{recursive:true});await writeFile(new URL('packages/api-client/src/identity.generated.js',root),output.js);await writeFile(new URL('packages/api-client/src/identity.generated.d.ts',root),output.dts)}
