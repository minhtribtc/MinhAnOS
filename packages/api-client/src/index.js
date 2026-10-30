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
