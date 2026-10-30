import {defineSettingsProvider} from '@minhanos/app-sdk'
import manifest from '../../../contracts/manifests/storage.app.json'
import DefaultsPanel from './DefaultsPanel.vue'

// Storage Manager configuration shown in central Settings; shares and quotas stay in the app itself.
export const settingsProvider=defineSettingsProvider({manifest,panels:{defaults:DefaultsPanel}})
