// Disposable test accounts are created through the v1 identity API (the legacy /api/admin/users adapter is
// gone): a fresh reauthentication, then POST /api/v1/identity/users with an Idempotency-Key.
import assert from 'node:assert/strict'
import {randomBytes} from 'node:crypto'
export async function createUser(base,admin,adminPassword,{username,password,displayName}){
 const headers={'Content-Type':'application/json',Cookie:admin.cookie,'X-CSRF-Token':admin.csrf}
 let r=await fetch(base+'/api/v1/identity/reauthenticate',{method:'POST',headers,body:JSON.stringify({password:adminPassword})})
 assert.equal(r.status,200,'reauthenticate')
 r=await fetch(base+'/api/v1/identity/users',{method:'POST',headers:{...headers,'Idempotency-Key':randomBytes(16).toString('hex')},body:JSON.stringify({username,displayName,password})})
 assert.equal(r.status,201,'create user')
 return r.json()
}
