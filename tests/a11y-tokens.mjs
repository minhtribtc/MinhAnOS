// Contrast of the design tokens (app standard row 22): text tokens reach WCAG AA 4.5:1 on every surface they sit
// on in light and dark, the focus ring 3:1; Desktop and Files never colour text with the grey accents or the
// focus colour (those fail AA). tests/a11y-browser.mjs checks the rendered pages with axe-core.
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const root=new URL('../',import.meta.url),text=file=>readFile(new URL(file,root),'utf8')
const css=await text('packages/ui/src/tokens.css')
const theme=sel=>{const block=css.slice(css.indexOf(sel));const body=block.slice(block.indexOf('{')+1,block.indexOf('}'));return Object.fromEntries([...body.matchAll(/--mn-color-([a-z-]+):(#[0-9a-f]{3,8})/g)].map(m=>[m[1],m[2]]))}
const rgb=h=>{h=h.slice(1);if(h.length===3)h=[...h].map(c=>c+c).join('');return [0,2,4].map(i=>parseInt(h.slice(i,i+2),16)/255)}
const lum=h=>{const [r,g,b]=rgb(h).map(c=>c<=.03928?c/12.92:((c+.055)/1.055)**2.4);return .2126*r+.7152*g+.0722*b}
const ratio=(a,b)=>{const [x,y]=[lum(a),lum(b)].sort((p,q)=>q-p);return (x+.05)/(y+.05)}
const surfaces=['surface','canvas','surface-raised','hover','selection']
for(const [name,sel] of [['light',':root'],['dark','[data-mn-theme="dark"]']]){
 const t=theme(sel),check=(fg,bg,min)=>assert(ratio(t[fg],t[bg])>=min,`${name}: ${fg} on ${bg} is ${ratio(t[fg],t[bg]).toFixed(2)}:1, needs ${min}:1`)
 for(const fg of ['text-primary','text-secondary','text-tertiary','action-primary','danger'])for(const bg of surfaces)check(fg,bg,4.5)
 check('on-action','action-primary',4.5);check('danger','danger-surface',4.5);check('success','success-surface',4.5);check('warning','warning-surface',4.5)
 for(const bg of ['surface','canvas','surface-raised'])check('focus',bg,3)
}
for(const file of ['packages/files-app/src/style.css','apps/desktop/src/style.css','apps/desktop/src/ipados.css','apps/desktop/src/fluent.css']){
 const bad=(await text(file)).match(/[;{]color:var\(--mn-(accent-slate-[0-9]+|color-focus)\)/g)?.length??0
 // Files keeps two icon-only uses (.file-symbol, .inspector-preview): icons need 3:1, which focus meets.
 assert(bad<=(file.includes('files-app')?2:0),file+': text coloured with a grey accent or the focus colour')
}
console.log('PASS accessibility tokens: AA contrast for text, actions and status colours in light and dark; focus ring 3:1')
