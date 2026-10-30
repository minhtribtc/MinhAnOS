// Storage Manager API types (/api/storage/...). Usage is the soft-quota ledger: logical bytes written
// through MinhAnOS; physicalBytes is what ZFS/Btrfs report for a share (null for plain volumes).
export interface Usage {limitBytes:number|null;usedBytes:number;reservedBytes:number;measured:boolean}
export interface Volume {name:string;kind:'zfs'|'btrfs'|'directory';managed:boolean;hardQuota:boolean;snapshots:boolean;online:boolean}
export interface Share {id:string;key:string;name:string;volume:string;kind:'share'|'homes';version:number;usage:Usage;physicalBytes:number|null}
export interface Home {userId:string;username:string;displayName:string;share:string;usage:Usage}

const GB=1024**3
/** Text in GB to bytes; empty means unlimited (null); undefined means invalid input. */
export function fromGB(value:string):number|null|undefined{
 const v=value.trim().replace(',','.')
 if(!v)return null
 const n=Number(v)
 return Number.isFinite(n)&&n>=0&&n<=1e9?Math.round(n*GB):undefined
}
export function toGB(bytes:number|null):string{return bytes==null?'':String(Math.round(bytes/GB*100)/100)}
