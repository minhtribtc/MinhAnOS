export type Request=<T=unknown>(path:string,options?:RequestInit)=>Promise<T>
export class ApiError extends Error {status:number;code:string;requestId?:string;constructor(status:number,body?:unknown)}
export function createApiClient(base?:string):{setCSRF(value:string):void;request:Request}
/** Random UUIDv4 for Idempotency-Key headers; works without a secure context (plain-HTTP LAN). */
export function createIdempotencyKey():string
