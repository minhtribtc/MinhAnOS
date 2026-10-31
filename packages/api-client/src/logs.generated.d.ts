// Generated from OpenAPI Logs operations. Do not edit.
import type {LogEventPage,LogSourceList} from '@minhanos/shared-types'
export function createLogsClient(base?:string):{setCSRF(value:string):void;logsListSources():Promise<LogSourceList>;logsListEvents(args?:{source?:string;outcome?:string;actor?:string;q?:string;limit?:number;cursor?:string;organization?:string}):Promise<LogEventPage>}
