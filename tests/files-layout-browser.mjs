// Files layout in a real browser: selecting an item never shifts the file list (the selection actions live in the
// fixed path row), and scrolling a long list moves only the list — the sidebar and the details column stay put.
// Checked at desktop width and at phone width (there the panes stack, scroll as one and the path row sticks). Creates folders layout-NN in the admin's home (left in place, like
// drive-smoke) on a running install; same environment as tests/a11y-browser.mjs.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {randomBytes} from 'node:crypto'
const {chromium}=await import(process.env.PLAYWRIGHT||'playwright')
const base=process.env.DRIVE_URL||'http://127.0.0.1:8082'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const browser=await chromium.launch()
for(const [width,height] of [[1280,800],[375,812]]){
 const context=await browser.newContext({viewport:{width,height},locale:'vi-VN'})
 const login=await context.request.post(base+'/api/login',{data:{username:'admin',password},headers:{Origin:base}})
 assert.equal(login.status(),200,'login')
 const {csrfToken}=await login.json()
 for(let i=0;i<40;i++){
  const name='layout-'+String(i).padStart(2,'0')
  const r=await context.request.post(base+'/api/files/mkdir?root=personal&path=',{data:{name},headers:{Origin:base,'X-CSRF-Token':csrfToken,'Idempotency-Key':randomBytes(16).toString('hex')}})
  assert([200,201,409].includes(r.status()),'mkdir '+name+' '+r.status())
 }
 const page=await context.newPage()
 await page.goto(base+'/');await page.waitForSelector('.files-app .file-item')
 const top=sel=>page.evaluate(s=>document.querySelector(s)?.getBoundingClientRect().top??null,sel)
 const first='.file-items .file-cell:first-child'
 const before=await top(first)
 await page.locator('.file-items .file-item').first().click()
 await page.waitForSelector('.selection-bar')
 assert.equal(await top(first),before,width+'px: selecting an item moved the list')
 if(width<700){
  // Narrow: the panes stack and scroll as one; the path row with the selection actions sticks to the top.
  const row=await page.evaluate(()=>{const l=document.querySelector('.drive-layout');l.scrollTop=l.scrollHeight;return {scrolled:l.scrollTop,row:Math.round(document.querySelector('.path-row').getBoundingClientRect().top),layout:Math.round(l.getBoundingClientRect().top)}})
  assert(row.scrolled>0&&row.row===row.layout,width+'px: the path row does not stick to the top '+JSON.stringify(row))
  await page.screenshot({path:`artifacts/files-layout-${width}.png`})
  console.log(`PASS Files layout ${width}px: selection keeps the list in place, the path row sticks while scrolling`)
  await context.close();continue
 }
 // Scroll the list itself; the page must not scroll and the side panes must not move.
 const panes=['.drive-sidebar','.file-inspector'],start=await Promise.all(panes.map(top))
 const scrolled=await page.evaluate(()=>{const b=document.querySelector('.file-browser');b.scrollTop=b.scrollHeight;return {list:b.scrollTop,page:document.scrollingElement.scrollTop}})
 assert(scrolled.list>0,width+'px: the file list does not scroll on its own')
 assert.equal(scrolled.page,0,width+'px: the page scrolled with the list')
 assert.deepEqual(await Promise.all(panes.map(top)),start,width+'px: sidebar or details moved while scrolling the list')
 await page.screenshot({path:`artifacts/files-layout-${width}.png`})
 console.log(`PASS Files layout ${width}px: selection keeps the list in place, only the list scrolls`)
 await context.close()
}
await browser.close()
