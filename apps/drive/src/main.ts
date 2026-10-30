import {createHost} from '@minhanos/app-sdk'
import '@minhanos/ui/tokens.css'
import {createApp} from 'vue'
import App from './App.vue'
import './style.css'
createApp(App).use(createHost()).mount('#app')
