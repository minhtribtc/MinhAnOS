export type ActionID='open'|'download'|'copy'|'cut'|'paste'|'rename'|'move'|'delete'|'properties'|'restore'|'mkdir'|'upload'|'refresh'|'grid'|'list'|'select-all'
export interface FileAction {id:ActionID;label:string;shortcut?:string;disabled?:boolean;danger?:boolean;divider?:boolean}
export interface ActionContext {kind:'empty'|'file'|'folder'|'special'|'trash';count:number;system:boolean;canPaste:boolean;canCopy:boolean}
// Menus, toolbar, overflow controls and keyboard all dispatch these same action IDs.
export function fileActions(c:ActionContext):FileAction[]{
 if(c.kind==='empty')return [{id:'mkdir',label:'Thư mục mới'},{id:'upload',label:'Tải lên'},{id:'paste',label:'Dán',shortcut:'Ctrl+V',disabled:!c.canPaste,divider:true},{id:'select-all',label:'Chọn tất cả',shortcut:'Ctrl+A'},{id:'grid',label:'Dạng lưới',divider:true},{id:'list',label:'Dạng danh sách'},{id:'refresh',label:'Làm mới'}]
 if(c.kind==='trash')return [{id:'restore',label:'Khôi phục'},{id:'properties',label:'Thuộc tính'}]
 return [{id:'open',label:c.kind==='folder'?'Mở thư mục':'Xem văn bản',shortcut:'Enter',disabled:c.count!==1||c.kind==='special'},
 {id:'download',label:'Tải xuống',disabled:c.count!==1||c.kind!=='file'},
 {id:'copy',label:'Sao chép',shortcut:'Ctrl+C',disabled:!c.canCopy,divider:true},
 {id:'cut',label:'Cắt',shortcut:'Ctrl+X',disabled:c.kind==='special'},
 {id:'paste',label:c.kind==='folder'?'Dán vào thư mục':'Dán',shortcut:'Ctrl+V',disabled:!c.canPaste||c.count!==1||c.kind!=='folder'},
 {id:'rename',label:'Đổi tên',shortcut:'F2',disabled:c.count!==1||c.kind==='special',divider:true},
 {id:'move',label:'Di chuyển đến…',disabled:c.count!==1||c.kind==='special'},
 {id:'delete',label:c.system?'Xóa vĩnh viễn…':'Đưa vào thùng rác',shortcut:'Delete',disabled:c.kind==='special',danger:c.system},
 {id:'properties',label:'Thuộc tính',divider:true}]
}
