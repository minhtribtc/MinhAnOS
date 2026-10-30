import assert from 'node:assert/strict'
import {readFile,writeFile,mkdir} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
const desktop='http://10.1.1.20:8081',drive='http://10.1.1.20:8082'
const password=(await readFile('/run/bootstrap_password','utf8')).trim()
class Client{
 cookie='';csrf=''
 constructor(base){this.base=base}
 async call(path,method='GET',body,token=true,origin){const headers={'Content-Type':'application/json',Cookie:this.cookie};if(token)headers['X-CSRF-Token']=this.csrf;if(origin)headers.Origin=origin;const response=await fetch(this.base+'/api'+path,{method,headers,body:body===undefined?undefined:JSON.stringify(body)});const cookie=response.headers.getSetCookie()[0];if(cookie)this.cookie=cookie.split(';')[0];return{status:response.status,body:await response.json()}}
 async login(username,password){const result=await this.call('/login','POST',{username,password});assert.equal(result.status,200);this.csrf=result.body.csrfToken;return result.body.user}
}
const guest=new Client(desktop);assert.equal((await guest.call('/profile')).body.code,'AUTH_REQUIRED')
const invalid=await guest.call('/login','POST',{username:'admin',password:'invalid-test-password'});assert.equal(invalid.body.code,'INVALID_CREDENTIALS')
const admin=new Client(desktop);await admin.login('admin',password)
const users=[];await mkdir('artifacts',{recursive:true})
const accounts=[]
for(let i=0;i<2;i++){const username='drive_e2e_'+randomBytes(4).toString('hex'),pwd=randomBytes(20).toString('hex');assert.equal((await admin.call('/admin/users','POST',{username,password:pwd,displayName:'Profile disposable test',role:'user'})).status,201);const client=new Client(desktop);const identity=await client.login(username,pwd);users.push({id:identity.id,username});await writeFile('artifacts/drive-test-users.json',JSON.stringify(users));accounts.push({client,username,pwd})}
const [a,b]=accounts;const profile={locale:'en-US',theme:'system',timeZone:'UTC'}
assert.equal((await a.client.call('/profile','PUT',profile,false)).status,403)
assert.equal((await a.client.call('/profile','PUT',profile,true,'https://evil.example')).status,403)
assert.equal((await a.client.call('/profile','PUT',{...profile,locale:'fr-FR'})).body.code,'PROFILE_INVALID')
assert.equal((await a.client.call('/profile','PUT',{...profile,timeZone:'invalid/zone'})).status,400)
assert.equal((await a.client.call('/profile','PUT',{})).status,400)
assert.equal((await a.client.call('/profile','PUT',profile)).status,200)
assert.deepEqual((await a.client.call('/drive/profile')).body,profile)
assert.equal((await b.client.call('/profile')).body.locale,'vi-VN')
const anotherDevice=new Client(desktop);await anotherDevice.login(a.username,a.pwd);assert.deepEqual((await anotherDevice.call('/profile')).body,profile)
const standalone=new Client(drive);await standalone.login(a.username,a.pwd);assert.deepEqual((await standalone.call('/profile')).body,profile)
const changed={locale:'vi-VN',theme:'dark',timeZone:'Asia/Bangkok'};assert.equal((await standalone.call('/profile','PUT',changed)).status,200);assert.deepEqual((await anotherDevice.call('/profile')).body,changed)
await anotherDevice.call('/logout','POST',{});assert.equal((await anotherDevice.call('/profile')).status,401)
console.log('PASS profile: two-user isolation, second-device persistence, standalone/native sharing, validation, stable codes, auth/CSRF/Origin/logout')
