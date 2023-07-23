<template>
  <div>

    <div class="mb-2 d-flex justify-content-between">
      <div class="h3">Repeater statistic</div>
      <div>
        <button class="btn btn-outline-secondary" @click="refreshList()">Refresh</button>
      </div>
    </div>

    <table class="table table-sm table-striped">
      <thead>
      <tr>
        <th>Topic / Group</th>
        <th>Count</th>
        <th>Last error</th>
        <th></th>
      </tr>
      </thead>
      <tbody>
      <tr v-for="item in list" :key="item.topic + ':' + item.group">
        <td>
          <div>{{ item.topic }}</div>
          <div class="small text-secondary">{{ item.group }}</div>
        </td>
        <td>{{ item.failedCount }} / {{ item.allCount }}</td>
        <td class="small">{{ item.lastError }}</td>
        <td class="d-flex justify-content-end align-items-stretch">
          <button class="btn btn-outline-secondary" @click="repeatTopicGroup(item.topic, item.group)">
            <img src="@/assets/repeat.svg" alt="Repeat all failed" width="30" height="30" title="Repeat all failed"/>
          </button>
          <button class="btn btn-outline-secondary ml-2" @click="getTopicGroupList(item.topic, item.group)">
            <img src="@/assets/list.svg" alt="Failed list" width="30" height="30" title="Failed list"/>
          </button>
        </td>
      </tr>
      </tbody>
    </table>

  </div>
</template>

<script lang="ts">
import {ref, defineComponent} from "vue";
import DataBusService, {RepeatStatResponse} from "@/services/DataBusService";
import {useNotification} from "@kyvg/vue3-notification";
import {useRequest} from "@/hooks/useRequest";

export default defineComponent({
  setup() {
    const list = ref({} as RepeatStatResponse['list'])
    const notification = useNotification()

    const refreshList = (dontNotify?: boolean) => {
      useRequest(DataBusService.getRepeatStat, (data: RepeatStatResponse['list']) => {
        list.value = data
        !dontNotify && notification.notify("Refreshed")
      })
    }

    const repeatTopicGroup = (topic: string, group: string) => {
      useRequest(() => DataBusService.repeatTopicGroup(topic, group), () => {
        refreshList(true)
      })
    }

    const getTopicGroupList = (topic: string, group: string) => {
      useRequest(() => DataBusService.repeatTopicGroup(topic, group), () => {
        refreshList(true)
      })
    }

    refreshList(true)

    return {list, refreshList, repeatTopicGroup, getTopicGroupList}
  }
})
</script>
