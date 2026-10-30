export type Request=<T=unknown>(path:string,options?:RequestInit)=>Promise<T>
export class ApiError extends Error {status:number;code:string;requestId?:string;constructor(status:number,body?:unknown)}
export function createApiClient(base?:string):{setCSRF(value:string):void;request:Request}
