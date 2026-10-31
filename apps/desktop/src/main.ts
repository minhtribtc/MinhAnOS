import {createHost} from '@minhanos/app-sdk'
import '@minhanos/ui/tokens.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import './fluent.css'

createApp(App).use(createPinia()).use(createHost()).mount('#app')
import './ipados.css'
import './dock.css'
