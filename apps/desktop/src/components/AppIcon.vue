<script setup lang="ts">
import { icons } from '../icons'
defineProps<{ id: string; color?: string; small?: boolean }>()
</script>
<template><span :class="['app-icon', color || 'mint', { small }]"><component :is="icons[id] || icons.app" :size="small ? 19 : 29" :stroke-width="1.8" /></span></template>
