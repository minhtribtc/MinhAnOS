import {createApiClient,createIdempotencyKey,ApiError} from '@minhanos/api-client'
import type {FilesEntry,FilesJob,FilesListPage,FilesRootList,FilesSession,FilesTrashItem} from '@minhanos/shared-types'
// DTOs are generated from contracts/files-api (OpenAPI); see packages/shared-types.
export type User=FilesSession['user']
export type Entry=FilesEntry
export type TrashItem=FilesTrashItem
// A background copy (folder, or file over the upload limit): GET /jobs/{id} for progress, POST .../cancel.
export type Job=FilesJob
// A storage root: the personal home, a Shared Folder (id "share:<key>", access read or write, its name) or the host
// system. Only Shared Folders carry a name; the UI names the home and the system root from its catalog.
export type Root=FilesRootList['roots'][number]
export function createFilesClient(base='/api'){
const client=createApiClient(base)
let csrf=''
function setCSRF(value:string){csrf=value;client.setCSRF(value)}
// Every write carries an Idempotency-Key, so a retried request is replayed instead of running twice (Files API contract).
const request=<T=any>(path:string,options:RequestInit={}):Promise<T>=>{if((options.method??'GET')!=='GET'){const headers=new Headers(options.headers);if(!headers.has('Idempotency-Key'))headers.set('Idempotency-Key',createIdempotencyKey());options={...options,headers}}return client.request<T>(path,options)}
// A folder listing comes in pages of at most 1000 entries; read them all (a folder holds at most 10,000).
async function list(root:string,path:string,get:<T>(path:string)=>Promise<T>=request){const entries:Entry[]=[];let cursor:string|null=null;do{const page:FilesListPage=await get(endpoint('list',root,path)+(cursor?'&cursor='+encodeURIComponent(cursor):''));entries.push(...page.entries);cursor=page.nextCursor}while(cursor);return entries}
function upload(root:string,path:string,file:File,onProgress:(value:number)=>void){
 const xhr=new XMLHttpRequest()
 const done=new Promise<void>((resolve,reject)=>{xhr.open('POST',`${base}${endpoint('upload',root,path,file.name)}`);xhr.withCredentials=true;xhr.setRequestHeader('X-CSRF-Token',csrf);xhr.setRequestHeader('Idempotency-Key',createIdempotencyKey());xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.upload.onprogress=e=>{if(e.lengthComputable)onProgress(Math.round(e.loaded/e.total*100))};xhr.onerror=()=>reject(new ApiError(0,{code:'NETWORK_ERROR'}));xhr.onabort=()=>reject(new ApiError(0,{code:'UPLOAD_ABORTED'}));xhr.onload=()=>{if(xhr.status>=200&&xhr.status<300)resolve();else reject(new ApiError(xhr.status,(()=>{try{return JSON.parse(xhr.responseText)}catch{return {code:'INVALID_RESPONSE'}}})()))};xhr.send(file)})
 return {xhr,done}
}

// The user's own data export (app standard row 17): the personal home as a tar archive.
return {request,list,setCSRF,upload,downloadURL:(root:string,path:string)=>base+endpoint('download',root,path),exportURL:base+'/export'}
}
export function endpoint(op:string,root:string,path:string,name?:string){return `/files/${op}?${new URLSearchParams({root,path,...name?{name}:{}})}`}
export function bytes(value:number){if(value<1024)return `${value} B`;if(value<1024**2)return `${(value/1024).toFixed(1)} KB`;if(value<1024**3)return `${(value/1024**2).toFixed(1)} MB`;return `${(value/1024**3).toFixed(1)} GB`}
