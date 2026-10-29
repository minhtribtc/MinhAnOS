export interface User {id:string;username:string;displayName:string;role:'admin'|'user'}
export interface Entry {name:string;path:string;kind:'file'|'folder'|'symlink'|'special';size:number;modified:string;mode:string;uid:number;gid:number;etag:string}
export interface TrashItem {id:string;path:string;name:string;deleted:string}
export interface Root {id:string;name:string}
let csrf=''
export function setCSRF(value:string){csrf=value}
export async function request<T>(path:string,options:RequestInit={}):Promise<T>{
 const response=await fetch(`/api${path}`,{...options,credentials:'same-origin',headers:{'Content-Type':'application/json','X-CSRF-Token':csrf,...options.headers}})
 const data=await response.json().catch(()=>({error:'Phản hồi không hợp lệ.'}))
 if(!response.ok)throw new Error(data.error||'Không thực hiện được yêu cầu.')
 return data
}
export function endpoint(op:string,root:string,path:string,name?:string){return `/files/${op}?${new URLSearchParams({root,path,...name?{name}:{}})}`}
export function bytes(value:number){if(value<1024)return `${value} B`;if(value<1024**2)return `${(value/1024).toFixed(1)} KB`;if(value<1024**3)return `${(value/1024**2).toFixed(1)} MB`;return `${(value/1024**3).toFixed(1)} GB`}
export function upload(root:string,path:string,file:File,onProgress:(value:number)=>void){
 const xhr=new XMLHttpRequest()
 const done=new Promise<void>((resolve,reject)=>{xhr.open('POST',`/api${endpoint('upload',root,path,file.name)}`);xhr.withCredentials=true;xhr.setRequestHeader('X-CSRF-Token',csrf);xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.upload.onprogress=e=>{if(e.lengthComputable)onProgress(Math.round(e.loaded/e.total*100))};xhr.onerror=()=>reject(new Error('Mất kết nối.'));xhr.onabort=()=>reject(new Error('Đã hủy.'));xhr.onload=()=>{if(xhr.status>=200&&xhr.status<300)resolve();else{let message='Tải lên thất bại.';try{message=JSON.parse(xhr.responseText).error||message}catch{}reject(new Error(message))}};xhr.send(file)})
 return {xhr,done}
}
