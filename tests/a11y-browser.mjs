// Accessibility in a real browser (app standard row 22) against a running install: axe-core WCAG 2.1 A/AA rules
// (names and labels, contrast, ARIA) on Desktop and Files in light and dark, dialogs that keep focus inside
// and close with Escape, and no horizontal overflow at phone width (375 px). Signs in through the API with the
// bootstrap admin (like api-smoke) and only opens and closes dialogs; it changes no data.
// Runs in the Playwright image: PLAYWRIGHT=<path to playwright index.mjs> AXE=<path to axe.min.js>.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const {chromium}=await import(process.env.PLAYWRIGHT||'playwright')
const axe=await readFile(process.env.AXE||'node_modules/axe-core/axe.min.js','utf8')
const desktopURL=process.env.TEST_URL||'http://127.0.0.1:8081',filesURL=process.env.DRIVE_URL||'http://127.0.0.1:8082'
const password=(await readFile(process.env.TEST_PASSWORD_FILE||'/run/bootstrap_password','utf8')).trim()
const files=JSON.parse(await readFile(new URL('../packages/files-app/locales/catalogs.json',import.meta.url),'utf8'))['vi-VN']
const browser=await chromium.launch()
const results=[]
const ok=name=>{console.log('PASS '+name);results.push(name)}
async function signedIn(base,{scheme='light',width=1280,height=860,path='/'}={}){
 // bypassCSP only lets the test inject axe; the app's CSP stays as served.
 const context=await browser.newContext({colorScheme:scheme,viewport:{width,height},locale:'vi-VN',bypassCSP:true})
 const r=await context.request.post(base+'/api/login',{data:{username:'admin',password},headers:{Origin:base}})
 assert.equal(r.status(),200,'login '+base)
 const page=await context.newPage()
 await page.goto(base+path);await page.waitForLoadState('networkidle')
 return {context,page}
}
async function audit(page,label){
 await page.addScriptTag({content:axe})
 const found=await page.evaluate(async()=>(await globalThis.axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}})).violations.map(v=>({id:v.id,impact:v.impact,nodes:v.nodes.slice(0,4).map(n=>n.target.join(' ')+' '+(n.failureSummary||'').split('\n')[1])})))
 assert.deepEqual(found,[],label+' axe violations:\n'+JSON.stringify(found,null,1))
}
async function focusStaysIn(page,dialog,label){
 for(let i=0;i<12;i++){await page.keyboard.press('Tab');assert(await page.evaluate(s=>!!document.activeElement?.closest(s),dialog),label+': Tab left the dialog')}
 for(let i=0;i<4;i++){await page.keyboard.press('Shift+Tab');assert(await page.evaluate(s=>!!document.activeElement?.closest(s),dialog),label+': Shift+Tab left the dialog')}
 await page.keyboard.press('Escape')
 await page.waitForFunction(s=>!document.querySelector(s),dialog)
}
const noOverflow=async(page,label)=>assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),label+' overflows horizontally at phone width')

for(const scheme of ['light','dark']){
 const {context,page}=await signedIn(desktopURL,{scheme})
 await audit(page,'Desktop '+scheme)
 await page.keyboard.press('Control+Space')
 const search='[role="dialog"][aria-modal="true"]'
 await page.waitForSelector(search)
 await audit(page,'Desktop search '+scheme)
 await focusStaysIn(page,search,'Desktop search')
 await context.close()
}
ok('Desktop: axe WCAG 2.1 AA in light and dark, search dialog keeps focus and closes with Escape')

for(const scheme of ['light','dark']){
 const {context,page}=await signedIn(filesURL,{scheme})
 await page.waitForSelector('.files-app .drive-main')
 await audit(page,'Files '+scheme)
 const trigger=page.getByRole('button',{name:files['files.actions.mkdir'],exact:true}).first()
 await trigger.click()
 const dialog='form[role="dialog"]'
 await page.waitForSelector(dialog)
 await audit(page,'Files dialog '+scheme)
 assert(await page.evaluate(s=>!!document.activeElement?.closest(s),dialog),'Files dialog takes focus')
 await focusStaysIn(page,dialog,'Files new-folder dialog')
 await context.close()
}
ok('Files: axe WCAG 2.1 AA in light and dark, new-folder dialog keeps focus and closes with Escape')

// Organization screens (row 27): Account Center → Tổ chức (tree, selected organization, members, new-organization
// dialog) and Storage Manager → Tổ chức (allocations). Opens and closes dialogs only.
const accounts=JSON.parse(await readFile(new URL('../packages/accounts-app/locales/catalogs.json',import.meta.url),'utf8'))['vi-VN']
const storageText=JSON.parse(await readFile(new URL('../packages/storage-app/locales/catalogs.json',import.meta.url),'utf8'))['vi-VN']
for(const scheme of ['light','dark']){
 const {context,page}=await signedIn(desktopURL,{scheme,path:'/apps/account-center'})
 await page.getByRole('button',{name:accounts['accounts.nav.orgs'],exact:true}).first().click()
 await page.locator('.mn-accounts .ac-entry').first().click()
 await page.getByRole('heading',{name:accounts['accounts.actions.members'],exact:true}).waitFor()
 await audit(page,'Account Center organizations '+scheme)
 await page.getByRole('button',{name:accounts['accounts.orgs.new']}).first().click()
 const dialog='.mn-accounts form[role="dialog"]'
 await page.waitForSelector(dialog)
 await audit(page,'Account Center organization dialog '+scheme)
 await focusStaysIn(page,dialog,'Account Center new-organization dialog')
 await page.goto(desktopURL+'/apps/storage');await page.waitForLoadState('networkidle')
 await page.getByRole('button',{name:storageText['storage.nav.orgs'],exact:true}).first().click()
 await page.locator('.mn-storage .sm-card').first().waitFor()
 await audit(page,'Storage Manager organizations '+scheme)
 await context.close()
}
ok('Account Center and Storage Manager organizations: axe WCAG 2.1 AA in light and dark, dialog keeps focus and closes with Escape')

for(const [base,label,path] of [[desktopURL,'Desktop','/'],[filesURL,'Files','/'],[desktopURL,'Account Center','/apps/account-center'],[desktopURL,'Storage Manager','/apps/storage']]){
 const {context,page}=await signedIn(base,{width:375,height:812,path})
 await noOverflow(page,label)
 await context.close()
}
ok('Desktop, Files, Account Center, Storage Manager: no horizontal overflow at 375 px')
await browser.close()
console.log(`a11y browser: ${results.length} checks passed`)
