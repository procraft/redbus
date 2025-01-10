<template>
  <div>

    <div class="mb-2 d-flex justify-content-between">
      <div class="h3">Topic statistic</div>
      <div>
        <button class="btn btn-outline-secondary" @click="refreshList()">Refresh</button>
      </div>
    </div>

    <table class="table table-sm table-striped">
      <thead>
      <tr>
        <th>Topic</th>
        <th>Partition</th>
        <th>Offset</th>
        <th>Group</th>
        <th>Offset</th>
        <th>Consumer</th>
        <th>State</th>
      </tr>
      </thead>
      <tbody>
      <tr v-for="topic in list" :key="topic.name + ':' + topic.partition.n + ':' + (topic.group?.name ?? '')">
        <td>{{ topic.name }}</td>
        <td>{{ topic.partition.n }}</td>
        <td>{{ topic.partition.firstOffset }} / {{ topic.partition.lastOffset }}</td>
        <td>{{ topic.group?.name }}</td>
        <td>{{ topic.group?.offset }}</td>
        <td>{{ topic.group?.consumerId }}</td>
        <td>{{ topic.group?.consumerState }}</td>
      </tr>
      </tbody>
    </table>

  </div>
</template>

<script lang="ts">
import {ref, defineComponent} from "vue";
import DataBusService, {TopicStatResponse} from "@/services/DataBusService";
import {useNotification} from "@kyvg/vue3-notification";
import {useRequest} from "@/hooks/useRequest";

export default defineComponent({
  setup() {
    type ListType = {
      name: string,
      partition?: TopicStatResponse['list'][0]['partitions'][0]
      group?: {name: string} & TopicStatResponse['list'][0]['groups'][0]['partitions'][0]
    }

    const list = ref({} as Array<ListType>)
    const notification = useNotification()

    const refreshList = (dontNotify?: boolean) => {
      useRequest(DataBusService.getTopicStat, (data: TopicStatResponse['list']) => {
        const result: Array<ListType> = []
        for (const topic of data) {
          for (const partition of topic.partitions) {
            let partitionAdded = false
            if (topic.groups?.length) {
              for (const group of topic.groups) {
                for (const groupPartition of group.partitions) {
                  if (groupPartition.n === partition.n) {
                    partitionAdded = true
                    result.push({
                      name: topic.name,
                      partition,
                      group: {name: group.name, ...groupPartition},
                    })
                  }
                }
              }
            }
            if (!partitionAdded) {
              result.push({
                name: topic.name,
                partition,
              })
            }
          }
        }
        console.log(result)
        list.value = result
        !dontNotify && notification.notify("Refreshed")
      })
    }

    refreshList(true)

    return {list, refreshList}
  }
})
</script>
