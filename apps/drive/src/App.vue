<script setup lang="ts">
import FilesApp,{PublicShare} from '@minhanos/files-app'
// /s/<token> is the page of a public link (ADR 0011); everything else is the Files app.
const publicLink=location.pathname.startsWith('/s/')
</script>
<template><PublicShare v-if="publicLink" /><FilesApp v-else /></template>
