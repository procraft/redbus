<template>
  <div id="app">
    <nav class="navbar navbar-expand navbar-dark bg-dark">
      <router-link to="/" class="navbar-brand">RED Bus control panel</router-link>
      <ul class="navbar-nav mr-auto">
        <!--        <li class="nav-item">-->
        <!--          <router-link to="/consumers" class="nav-link">Consumers</router-link>-->
        <!--        </li>-->
        <!--        <li class="nav-item">-->
        <!--          <router-link to="/repeater" class="nav-link">Repeater</router-link>-->
        <!--        </li>-->
      </ul>
    </nav>

    <div class="container mt-3">
      <router-view/>
    </div>
    <notifications position="bottom right"/>
  </div>
</template>

<script lang="ts">
import {defineComponent} from "vue";

export default defineComponent({
  name: "App",
});
</script>
