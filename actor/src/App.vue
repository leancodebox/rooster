<script setup lang="ts">
import JobManager from "@/views/JobManager.vue";
import {dateZhCN, NCard, NConfigProvider, NMessageProvider, zhCN} from 'naive-ui'
</script>

<template>
  <!--  <header>-->
  <!--    <img alt="Vue logo" class="logo" src="@/assets/logo.svg" width="125" height="125" />-->

  <!--    <div class="wrapper">-->
  <!--      <HelloWorld msg="You did it!" />-->

  <!--      <nav>-->
  <!--        <RouterLink to="/">Home</RouterLink>-->
  <!--        <RouterLink to="/about">About</RouterLink>-->
  <!--      </nav>-->
  <!--    </div>-->
  <!--  </header>-->

  <!--  <RouterView />-->

  <n-message-provider>
    <n-config-provider :locale="zhCN" :date-locale="dateZhCN">
      <n-card title="Task Manager"  :bordered="false">
        <JobManager></JobManager>
      </n-card>
    </n-config-provider>
  </n-message-provider>
</template>

<style scoped>

</style>
