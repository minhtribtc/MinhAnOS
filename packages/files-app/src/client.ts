import {createApiClient,ApiError} from '@minhanos/app-sdk'
export interface User {id:string;username:string;displayName:string;role:'admin'|'user'}
export interface Entry {name:string;path:string;kind:'file'|'folder'|'symlink'|'special';size:number;modified:string;mode:string;uid:number;gid:number;etag:string}
export interface TrashItem {id:string;path:string;name:string;deleted:string}
export interface Root {id:string;name:string}
export function createFilesClient(base='/api'){
const client=createApiClient(base)
let csrf=''
function setCSRF(value:string){csrf=value;client.setCSRF(value)}
const request=client.request
function upload(root:string,path:string,file:File,onProgress:(value:number)=>void){
 const xhr=new XMLHttpRequest()
 const done=new Promise<void>((resolve,reject)=>{xhr.open('POST',`${base}${endpoint('upload',root,path,file.name)}`);xhr.withCredentials=true;xhr.setRequestHeader('X-CSRF-Token',csrf);xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.upload.onprogress=e=>{if(e.lengthComputable)onProgress(Math.round(e.loaded/e.total*100))};xhr.onerror=()=>reject(new Error('Mất kết nối.'));xhr.onabort=()=>reject(new Error('Đã hủy.'));xhr.onload=()=>{if(xhr.status>=200&&xhr.status<300)resolve();else{let message='Tải lên thất bại.';try{message=JSON.parse(xhr.responseText).error||message}catch{}reject(new ApiError(xhr.status,(()=>{try{return JSON.parse(xhr.responseText)}catch{return {error:message,code:'INVALID_RESPONSE'}}})()))}};xhr.send(file)})
 return {xhr,done}
}

return {request,setCSRF,upload,downloadURL:(root:string,path:string)=>base+endpoint('download',root,path)}
}
export function endpoint(op:string,root:string,path:string,name?:string){return `/files/${op}?${new URLSearchParams({root,path,...name?{name}:{}})}`}
export function bytes(value:number){if(value<1024)return `${value} B`;if(value<1024**2)return `${(value/1024).toFixed(1)} KB`;if(value<1024**3)return `${(value/1024**2).toFixed(1)} MB`;return `${(value/1024**3).toFixed(1)} GB`}
