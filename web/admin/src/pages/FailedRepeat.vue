<template>
  <div>
    <DashboardStat/>
    <RepeatList/>
  </div>
</template>

<script lang="ts">
import {defineComponent} from "vue";
import DashboardStat from "@/components/DashboardStat.vue";
import RepeatList from "@/components/RepeatList.vue";

export default defineComponent({
  components: {RepeatList, DashboardStat},
})
</script>
