export class ApiError extends Error {
 constructor(status,body={}){const payload=body?.error&&typeof body.error==='object'?body.error:body||{};super(typeof payload.message==='string'?payload.message:typeof body?.error==='string'?body.error:'Request failed');this.name='ApiError';this.status=status;this.code=typeof payload.code==='string'?payload.code:'REQUEST_FAILED';this.requestId=typeof payload.requestId==='string'?payload.requestId:undefined}
}
export function createApiClient(base='/api'){
 let csrf=''
 return {setCSRF(value){csrf=value},async request(path,options={}){
  const headers=new Headers(options.headers);if(!headers.has('Content-Type'))headers.set('Content-Type','application/json');headers.set('X-CSRF-Token',csrf)
  const response=await fetch(base+path,{...options,credentials:'same-origin',headers})
  const body=await response.json().catch(()=>({code:'INVALID_RESPONSE'}))
  if(!response.ok)throw new ApiError(response.status,body)
  return body
 }}
}
// Idempotency keys must also work on plain-HTTP LAN installs: crypto.randomUUID exists only in
// secure contexts (HTTPS/localhost), crypto.getRandomValues works everywhere. Returns a UUIDv4.
export function createIdempotencyKey(){
 if(typeof globalThis.crypto?.randomUUID==='function')return globalThis.crypto.randomUUID()
 const b=globalThis.crypto.getRandomValues(new Uint8Array(16));b[6]=b[6]&0x0f|0x40;b[8]=b[8]&0x3f|0x80
 const h=[...b].map(x=>x.toString(16).padStart(2,'0')).join('')
 return h.slice(0,8)+'-'+h.slice(8,12)+'-'+h.slice(12,16)+'-'+h.slice(16,20)+'-'+h.slice(20)
}
