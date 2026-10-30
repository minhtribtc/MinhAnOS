/** Deep-link request: open Settings at an app panel. `seq` changes on every request. */
export interface SettingsTarget {appId:string;panelId?:string;seq:number}
