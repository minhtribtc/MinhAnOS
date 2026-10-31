// Files sharing (ADR 0011) on a running install: the admin shares a folder of the home with a second account of the
// same organization (view, then edit), and by public links (password + view, drop); the second account opens it as
// a shared:<id> root; an anonymous visitor uses the links. Then the UI: Share dialog, "Shared by me", "Shared with me"
// and the public page. Same environment as tests/files-manage-browser.mjs; creates the account e2e_share_<random>.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
const {chromium}=await import(process.env.PLAYWRIGHT||'playwright')
const base=process.env.DRIVE_URL||'http://127.0.0.1:8082',desktop=process.env.TEST_URL||'http://127.0.0.1:8081'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const key=()=>randomBytes(16).toString('hex')
const browser=await chromium.launch()
const newContext=()=>browser.newContext({viewport:{width:1280,height:800},locale:'vi-VN'})
// A signed-in API client of one origin; post() checks the status.
async function signIn(origin,username,secret){
 const context=await newContext(),api=context.request
 const r=await api.post(origin+'/api/login',{data:{username,password:secret},headers:{Origin:origin}})
 assert.equal(r.status(),200,'login '+username)
 const {csrfToken}=await r.json()
 const headers=()=>({Origin:origin,'X-CSRF-Token':csrfToken,'Idempotency-Key':key()})
 const post=async(path,data,status=200)=>{const res=await api.post(origin+path,{data,headers:headers()});assert.equal(res.status(),status,path+' '+await res.text());return res.json()}
 const upload=(path,name,body,root='personal')=>api.post(`${origin}/api/files/upload?root=${encodeURIComponent(root)}&path=${encodeURIComponent(path)}&name=${encodeURIComponent(name)}`,{data:Buffer.from(body),headers:{...headers(),'Content-Type':'application/octet-stream'}})
 const get=async path=>{const res=await api.get(origin+path);return {status:res.status(),body:await res.json().catch(()=>null)}}
 return {context,api,post,upload,get,headers}
}
// The second account, in the admin's first organization.
const admin=await signIn(desktop,'admin',password)
await admin.post('/api/v1/identity/reauthenticate',{password})
const bobName='e2e_share_'+randomBytes(4).toString('hex'),bobPassword=randomBytes(18).toString('hex')
await admin.post('/api/v1/identity/users',{username:bobName,displayName:'Bob Share',password:bobPassword},201)
const orgs=(await admin.get('/api/v1/identity/organizations')).body.items
const joined=await admin.api.post(`${desktop}/api/v1/identity/organizations/${orgs[0].id}/members`,{data:{username:bobName,role:'member'},headers:admin.headers()})
assert([201,409].includes(joined.status()),'join organization '+await joined.text())
await admin.context.close()

const owner=await signIn(base,'admin',password),bob=await signIn(base,bobName,bobPassword)
const dir='share-'+randomBytes(4).toString('hex')
await owner.post(`/api/files/mkdir?root=personal&path=`,{name:dir})
assert.equal((await owner.upload(dir,'hello.txt','hello')).status(),201)
const people=(await owner.get('/api/sharing/recipients?q='+bobName)).body.items
assert.equal(people.length,1,'the colleague is found');assert(!('admin' in people[0]))
const grant=await owner.post('/api/sharing/grants',{path:dir,recipient:people[0].id,permission:'view'})
const received=(await bob.get('/api/sharing/received')).body.items
assert(received.some(g=>g.id===grant.id&&g.name===dir),'listed under Shared with me')
const root='shared:'+grant.id
const listed=(await bob.get(`/api/files/list?root=${root}&path=`)).body
assert.deepEqual(listed.entries.map(e=>e.name),['hello.txt'],'the shared folder content, nothing else')
assert([400,404].includes((await bob.get(`/api/files/list?root=${root}&path=..`)).status),'no way out of the shared folder')
const denied=await bob.upload('','x.txt','x',root);assert.equal(denied.status(),403,'view grant refuses writes');assert.equal((await denied.json()).code,'SHARE_READ_ONLY')
await owner.post('/api/sharing/grants',{path:dir,recipient:people[0].id,permission:'edit'})
assert.equal((await bob.upload('','from-bob.txt','bob',root)).status(),201,'edit grant writes into the owner home')
assert((await owner.get(`/api/files/list?root=personal&path=${dir}`)).body.entries.some(e=>e.name==='from-bob.txt'))
assert.equal((await bob.get(`/api/files/trash?root=${root}&path=`)).status,403,'no trash of a shared root')
// Favorites follow a rename by the owner and go with a delete.
await bob.post('/api/favorites',{root,path:'hello.txt',dir:false})
await owner.post(`/api/files/rename?root=personal&path=${encodeURIComponent(dir+'/hello.txt')}`,{name:'hi.txt'})
assert.deepEqual((await bob.get('/api/favorites')).body.items.filter(f=>f.root===root).map(f=>f.path),['hi.txt'],'favorite follows the rename')
await owner.post(`/api/files/trash?root=personal&path=${encodeURIComponent(dir+'/from-bob.txt')}`,{})
console.log('PASS Files sharing with people: recipient search, view/edit grants, scoped shared root, favorites follow renames')

const linkPassword=randomBytes(9).toString('hex')
const view=await owner.post('/api/sharing/links',{path:dir,permission:'view',password:linkPassword,expires:new Date(Date.now()+86400e3).toISOString()})
const drop=await owner.post('/api/sharing/links',{path:dir,permission:'drop'})
assert.equal(view.token.length,64);assert(view.hasPassword)
const visitor=await newContext(),pub=(token,op='')=>`${base}/api/public/${token}${op}`
let info=await (await visitor.request.get(pub(view.token))).json()
assert.equal(info.needsPassword,true);assert.equal(info.owner.length>0,true)
assert.equal((await visitor.request.get(pub(view.token,'/files/list?path='))).status(),401,'locked until the password')
assert.equal((await visitor.request.post(pub(view.token,'/unlock'),{data:{password:'wrong-password'},headers:{Origin:base}})).status(),403)
assert.equal((await visitor.request.post(pub(view.token,'/unlock'),{data:{password:linkPassword},headers:{Origin:base}})).status(),200)
const page1=await (await visitor.request.get(pub(view.token,'/files/list?path='))).json()
assert.deepEqual(page1.entries.map(e=>e.name),['hi.txt'])
assert.equal(await (await visitor.request.get(pub(view.token,'/files/download?path=hi.txt'))).text(),'hello')
assert.equal((await visitor.request.post(pub(view.token,'/files/upload?path=&name=v.txt'),{data:Buffer.from('v'),headers:{Origin:base,'Content-Type':'application/octet-stream'}})).status(),403,'view link refuses uploads')
assert.equal((await visitor.request.get(pub(drop.token,'/files/list?path='))).status(),403,'a drop link lists nothing')
assert.equal((await visitor.request.post(pub(drop.token,'/files/upload?path=&name=dropped.txt'),{data:Buffer.from('drop'),headers:{Origin:base,'Content-Type':'application/octet-stream'}})).status(),201,'drop link uploads')
assert.equal((await visitor.request.post(pub(drop.token,'/files/upload?path=&name=x.txt'),{data:Buffer.from('x'),headers:{Origin:'http://evil.example','Content-Type':'application/octet-stream'}})).status(),403,'cross-site writes refused')
assert((await owner.get(`/api/files/list?root=personal&path=${dir}`)).body.entries.some(e=>e.name==='dropped.txt'))
assert.equal((await visitor.request.get(pub('0'.repeat(64)))).status(),404)
const links=(await owner.get('/api/sharing?path='+dir)).body.links
assert(links.find(l=>l.id===view.id).uses>=1,'uses counted')
console.log('PASS Files public links: password unlock, view, drop-only upload, cross-site refused, use count')

// UI: the owner sees the share mark, the dialog and Shared by me; the colleague Shared with me; the visitor the page.
const ownerPage=await owner.context.newPage()
await ownerPage.goto(base+'/');await ownerPage.waitForSelector('.files-app .file-item')
const item=ownerPage.locator('.file-item',{hasText:dir})
await item.locator('.share-mark').waitFor()
await item.click({button:'right'})
await ownerPage.getByRole('menuitem',{name:'Chia sẻ'}).click()
const dialog=ownerPage.getByRole('dialog',{name:`Chia sẻ “${dir}”`})
await dialog.waitFor()
await dialog.getByText('Bob Share').waitFor()
assert.equal(await dialog.locator('.share-link-url code').count(),2,'both links listed')
await dialog.locator('header button').click()
await ownerPage.getByRole('button',{name:'Đã chia sẻ',exact:true}).click()
await ownerPage.locator('.favorite-row',{hasText:dir}).waitFor()
const bobPage=await bob.context.newPage()
await bobPage.goto(base+'/');await bobPage.waitForSelector('.drive-sidebar')
await bobPage.getByRole('button',{name:/Chia sẻ với tôi/}).click()
await bobPage.locator('.favorite-row',{hasText:dir}).click()
await bobPage.locator('.file-item',{hasText:'hi.txt'}).waitFor()
const publicPage=await (await newContext()).newPage()
await publicPage.goto(base+'/s/'+view.token)
await publicPage.getByRole('heading',{name:dir}).waitFor()
await publicPage.getByLabel('Mật khẩu').fill(linkPassword)
await publicPage.getByRole('button',{name:'Mở'}).click()
await publicPage.getByText('hi.txt').waitFor()
const missing=await (await newContext()).newPage()
await missing.goto(base+'/s/'+'f'.repeat(64))
await missing.getByRole('heading',{name:'Không tìm thấy liên kết'}).waitFor()
console.log('PASS Files sharing UI: share mark, Share dialog, Shared by me, Shared with me, public page unlock and listing')

// Deleting the folder removes its grants and links.
await owner.post(`/api/files/trash?root=personal&path=${encodeURIComponent(dir)}`,{})
const after=(await owner.get('/api/sharing?path='+dir)).body
assert.equal(after.grants.length+after.links.length,0,'shares go with the item')
assert.equal((await visitor.request.get(pub(view.token))).status(),404,'its links stop working')
await browser.close()
