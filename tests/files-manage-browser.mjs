// Files management (roadmap phase 1) on a running install: server search over subfolders, recent files, a ZIP of
// several items, favorites, permanent delete and emptying the trash through the API, then the new controls in the
// UI (sort, Recent, Favorites, Upload folder). Same environment as tests/files-layout-browser.mjs; works in the
// admin's home under manage-<random>.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
const {chromium}=await import(process.env.PLAYWRIGHT||'playwright')
const base=process.env.DRIVE_URL||'http://127.0.0.1:8082'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const browser=await chromium.launch()
const context=await browser.newContext({viewport:{width:1280,height:800},locale:'vi-VN'})
const api=context.request
const login=await api.post(base+'/api/login',{data:{username:'admin',password},headers:{Origin:base}})
assert.equal(login.status(),200,'login')
const {csrfToken}=await login.json()
const headers=()=>({Origin:base,'X-CSRF-Token':csrfToken,'Idempotency-Key':randomBytes(16).toString('hex')})
const q=(op,path,extra='')=>`${base}/api/files/${op}?root=personal&path=${encodeURIComponent(path)}${extra}`
const post=async(url,data,status=200)=>{const r=await api.post(url,{data,headers:headers()});assert.equal(r.status(),status,url+' '+await r.text());return r.json()}
const dir='manage-'+randomBytes(4).toString('hex')
await post(q('mkdir',''),{name:dir})
await post(q('mkdir',dir),{name:'Sub'})
for(const [folder,name,body] of [[dir,'Report-2026.txt','a'],[dir+'/Sub','report-old.txt','bb'],[dir,'notes.md','ccc']]){
 const r=await api.post(q('upload',folder,'&name='+name),{data:Buffer.from(body),headers:{...headers(),'Content-Type':'application/octet-stream'}})
 assert.equal(r.status(),201,'upload '+name)
}
const found=await (await api.get(q('search',dir,'&q=REPORT'))).json()
assert.deepEqual(found.entries.map(e=>e.path).sort(),[dir+'/Report-2026.txt',dir+'/Sub/report-old.txt'],'search covers subfolders, case-insensitive')
const recent=await (await api.get(q('recent',dir))).json()
assert.equal(recent.entries.length,3,'recent lists the three files');assert(recent.entries.every(e=>e.kind==='file'))
const zip=await api.get(`${base}/api/files/archive?root=personal&path=&item=${encodeURIComponent(dir+'/Sub')}&item=${encodeURIComponent(dir+'/notes.md')}`)
assert.equal(zip.status(),200);assert.equal(zip.headers()['content-type'],'application/zip')
const bytes=await zip.body();assert.equal(bytes.subarray(0,2).toString(),'PK','zip body')
assert(bytes.includes(Buffer.from('Sub/report-old.txt'))&&bytes.includes(Buffer.from('notes.md')),'zip holds the folder tree and the file')
await post(base+'/api/favorites',{root:'personal',path:dir+'/Sub',dir:true})
assert((await (await api.get(base+'/api/favorites')).json()).items.some(f=>f.path===dir+'/Sub'),'favorite listed')
const one=(await post(q('trash',dir+'/notes.md'),{})).item
await post(q('trash',dir+'/Report-2026.txt'),{})
const purged=await post(q('trash-delete',''),{id:one.id})
assert(purged.freedBytes>=3,'trash-delete frees the file');assert(!(await (await api.get(q('trash',''))).json()).some(i=>i.id===one.id))
await post(q('empty-trash',''),{});assert.equal((await (await api.get(q('trash',''))).json()).length,0,'trash emptied')
console.log('PASS Files management API: subtree search, recent, ZIP of several items, favorites, permanent delete, empty trash')
const page=await context.newPage()
await page.goto(base+'/');await page.waitForSelector('.files-app .file-item')
await page.selectOption('.sort-control select','size')
assert.equal(await page.evaluate(()=>localStorage.getItem('drive.sort')),'size:asc','sort remembered')
assert(await page.getByRole('button',{name:'Tải thư mục lên'}).isVisible(),'upload folder button')
await page.getByRole('button',{name:'Yêu thích',exact:true}).click()
await page.getByText('Sub',{exact:true}).first().waitFor()
await page.getByRole('button',{name:'Gần đây',exact:true}).click()
await page.locator('.drive-heading h1',{hasText:'Gần đây'}).waitFor()
await post(base+'/api/favorites/remove',{root:'personal',path:dir+'/Sub'})
console.log('PASS Files management UI: sort is remembered, Upload folder, Favorites and Recent views open')
await browser.close()
