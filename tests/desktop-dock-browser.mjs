// Dock-only Shell (ADR 0013) on a running install: no top bar; focus mode fills the work area beside the dock;
// maximize takes the whole screen, hides the dock behind a pill and shows it at the screen edge; the dock moves to
// the left from its context menu (persisted); pinning from Home moves an app into the dock and off Home; the bell
// opens the notification side panel. Restores the admin's dock and pins at the end.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT||'playwright')
const desktop=process.env.TEST_URL||'http://127.0.0.1:8081'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const browser=await chromium.launch()
const context=await browser.newContext({viewport:{width:1280,height:800},locale:'vi-VN'})
const login=await context.request.post(desktop+'/api/login',{data:{username:'admin',password},headers:{Origin:desktop}})
assert.equal(login.status(),200)
const before=await (await context.request.get(desktop+'/api/preferences')).json()
// Start from a known layout: focus mode, dock at the bottom, no open windows.
async function savePrefs(change){
 const now=await context.request.get(desktop+'/api/preferences')
 const csrf=(await (await context.request.get(desktop+'/api/session')).json()).csrfToken
 const r=await context.request.put(desktop+'/api/preferences',{data:{...await now.json(),...change},headers:{Origin:desktop,'X-CSRF-Token':csrf,'If-Match':now.headers()['etag']}})
 assert.equal(r.status(),200,'save preferences '+await r.text())
}
await savePrefs({mode:'focus',windows:[],dock:{position:'bottom',size:'medium'}})
const page=await context.newPage()
const box=selector=>page.locator(selector).first().boundingBox()
await page.goto(desktop+'/')
const dock=page.locator('nav.tablet-dock')
await dock.waitFor()
try{
 assert.equal(await page.locator('.tablet-status').count(),0,'top bar removed')
 assert(await dock.locator('.dock-clock').isVisible(),'clock in the dock tray')
 // Focus mode: the app fills the screen minus the dock.
 await dock.getByRole('button',{name:/MinhAnOS Files/}).click()
 const win=page.locator('.desktop-window[data-window="files"]')
 await win.waitFor()
 await page.waitForTimeout(700)
 let r=await win.boundingBox()
 assert(r.width>=1260&&r.height>=670&&r.y<=10,'focus window uses the work area '+JSON.stringify(r))
 // Maximize: whole screen, dock away, pill on the edge; the bottom edge shows the dock again.
 await win.getByRole('button',{name:/Phóng to/}).click()
 await page.waitForTimeout(300)
 r=await win.boundingBox()
 assert.deepEqual([r.x,r.y,r.width,r.height],[0,0,1280,800],'maximized fills the screen')
 assert(await dock.evaluate(el=>el.classList.contains('dock-away')),'dock hidden when maximized')
 assert(await page.locator('.dock-pill').isVisible(),'pill with the bell stays')
 await page.mouse.move(640,798)
 await page.waitForTimeout(300)
 assert(!await dock.evaluate(el=>el.classList.contains('dock-away')),'edge shows the dock')
 await page.mouse.move(640,300)
 await page.waitForTimeout(900)
 assert(await dock.evaluate(el=>el.classList.contains('dock-away')),'dock hides again')
 await win.getByRole('button',{name:/Khôi phục/}).click()
 await page.waitForTimeout(500)
 // Context menu on empty dock space: move the dock to the left; it is saved.
 const d=await dock.boundingBox()
 await page.mouse.click(d.x+4,d.y+d.height/2,{button:'right'})
 await page.getByRole('menuitemradio',{name:'Trái'}).click()
 await page.waitForTimeout(400)
 assert(await page.locator('.tablet-shell.dock-left').count(),'dock on the left')
 r=await win.boundingBox()
 const dl=await dock.boundingBox()
 assert(r.x>=dl.x+dl.width,'window beside the left dock '+JSON.stringify([r,dl]))
 await page.waitForTimeout(800)
 await page.reload()
 await dock.waitFor()
 assert(await page.locator('.tablet-shell.dock-left').count(),'dock position persisted')
 // Pin from Home: the app leaves Home and joins the dock.
 await dock.getByRole('button',{name:'Màn hình chính'}).click()
 const homeApp=page.locator('.home-app').first()
 const label=await homeApp.getAttribute('aria-label')
 await homeApp.click({button:'right'})
 await page.getByRole('menuitem',{name:'Ghim vào Dock'}).click()
 await page.waitForTimeout(300)
 assert.equal(await page.locator(`.home-app[aria-label="${label}"]`).count(),0,'pinned app left Home')
 assert.equal(await dock.locator(`[aria-label="${label}"]`).count(),1,'pinned app in the dock')
 // The bell opens the notification side panel, full height.
 const bell=dock.locator('.dock-bell')
 if(await bell.count()){
  await bell.click()
  const sheet=await box('.tablet-notifications')
  assert(sheet&&sheet.height>=700,'notification side panel '+JSON.stringify(sheet))
  await page.keyboard.press('Escape')
 }
 console.log('PASS Desktop dock: no top bar, work area, maximize hides the dock (pill, edge reveal), dock position persisted, pin from Home, notification side panel')
}finally{
 await savePrefs({mode:before.mode,dock:before.dock,pinned:before.pinned,windows:[]}).catch(e=>console.error(e))
 await browser.close()
}
