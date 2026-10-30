import assert from 'node:assert/strict'
import {fileActions} from '../packages/files-app/src/actions.ts'
const base={kind:'file',count:1,system:false,canPaste:false,canCopy:true}
assert.equal(fileActions(base).find(a=>a.id==='delete').label,'Đưa vào thùng rác')
assert.equal(fileActions({...base,system:true}).find(a=>a.id==='delete').label,'Xóa vĩnh viễn…')
assert.equal(fileActions({...base,kind:'folder',canCopy:false}).find(a=>a.id==='copy').disabled,true)
assert.equal(fileActions({...base,count:2}).find(a=>a.id==='rename').disabled,true)
assert.equal(fileActions({...base,kind:'empty'}).find(a=>a.id==='paste').disabled,true)
assert.equal(fileActions({...base,kind:'trash'}).some(a=>a.id==='restore'),true)
console.log('PASS Action registry: system delete, folder copy limits, multi-rename, empty clipboard and trash actions')
