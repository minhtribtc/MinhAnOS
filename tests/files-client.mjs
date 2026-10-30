import assert from 'node:assert/strict'
import {createFilesClient} from '../packages/files-app/src/client.ts'
const seen=[];const original=globalThis.fetch
try{globalThis.fetch=async(url,options)=>{seen.push({url,headers:options.headers});return {ok:true,json:async()=>({ok:true})}}
const desktop=createFilesClient('/api/drive'),standalone=createFilesClient('/api');desktop.setCSRF('desktop-csrf');standalone.setCSRF('drive-csrf')
await desktop.request('/session');await standalone.request('/session')
assert.equal(seen[0].url,'/api/drive/session');assert.equal(seen[1].url,'/api/session')
assert.equal(seen[0].headers['X-CSRF-Token'],'desktop-csrf');assert.equal(seen[1].headers['X-CSRF-Token'],'drive-csrf')
assert.match(desktop.downloadURL('personal','Tiếng Việt.txt'),/^\/api\/drive\/files\/download\?/)
console.log('PASS Shared Files client keeps API endpoints and CSRF state isolated by instance')
}finally{globalThis.fetch=original}
