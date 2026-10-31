import {createApiClient,ApiError} from '@minhanos/api-client'
export interface User {id:string;username:string;displayName:string;role:'admin'|'user'}
export interface Entry {name:string;path:string;kind:'file'|'folder'|'symlink'|'special';size:number;modified:string;mode:string;uid:number;gid:number;etag:string}
export interface TrashItem {id:string;path:string;name:string;deleted:string}
// A storage root: the personal home, a Shared Folder (id "share:<key>", access read or write) or the host system.
export interface Root {id:string;name:string;kind?:'home'|'share'|'system';access?:'read'|'write'}
export function createFilesClient(base='/api'){
const client=createApiClient(base)
let csrf=''
function setCSRF(value:string){csrf=value;client.setCSRF(value)}
const request=client.request
function upload(root:string,path:string,file:File,onProgress:(value:number)=>void){
 const xhr=new XMLHttpRequest()
 const done=new Promise<void>((resolve,reject)=>{xhr.open('POST',`${base}${endpoint('upload',root,path,file.name)}`);xhr.withCredentials=true;xhr.setRequestHeader('X-CSRF-Token',csrf);xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.upload.onprogress=e=>{if(e.lengthComputable)onProgress(Math.round(e.loaded/e.total*100))};xhr.onerror=()=>reject(new ApiError(0,{code:'NETWORK_ERROR'}));xhr.onabort=()=>reject(new ApiError(0,{code:'UPLOAD_ABORTED'}));xhr.onload=()=>{if(xhr.status>=200&&xhr.status<300)resolve();else reject(new ApiError(xhr.status,(()=>{try{return JSON.parse(xhr.responseText)}catch{return {code:'INVALID_RESPONSE'}}})()))};xhr.send(file)})
 return {xhr,done}
}

return {request,setCSRF,upload,downloadURL:(root:string,path:string)=>base+endpoint('download',root,path)}
}
export function endpoint(op:string,root:string,path:string,name?:string){return `/files/${op}?${new URLSearchParams({root,path,...name?{name}:{}})}`}
export function bytes(value:number){if(value<1024)return `${value} B`;if(value<1024**2)return `${(value/1024).toFixed(1)} KB`;if(value<1024**3)return `${(value/1024**2).toFixed(1)} MB`;return `${(value/1024**3).toFixed(1)} GB`}
