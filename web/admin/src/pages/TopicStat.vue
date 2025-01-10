<template>
  <div>
    <DashboardStat/>
    <TopicList/>
  </div>
</template>

<script lang="ts">
import {defineComponent} from "vue";
import DashboardStat from "@/components/DashboardStat.vue";
import TopicList from "@/components/TopicList.vue";

export default defineComponent({
  components: {TopicList, DashboardStat},
})
</script>
