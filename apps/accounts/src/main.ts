import {createApp} from 'vue'
import {createHost} from '@minhanos/app-sdk'
import '@minhanos/ui/tokens.css'
import AccountsApp from '@minhanos/accounts-app'
import './style.css'
createApp(AccountsApp).use(createHost()).mount('#app')
