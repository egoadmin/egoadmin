<route lang="yaml">
meta:
  enabled: false
</route>

<template>
  <page-main>
    <router-link :to="{ name: 'breadcrumbExampleDetail2' }">
      {{ $t('breadcrumb.viewDetail') }}
    </router-link>
  </page-main>
</template>
