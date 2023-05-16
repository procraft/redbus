<template>
    <div class="row mb-3">
        <div class="col-lg-4 col-md-6 col-sm-12 py-2">
            <div class="card bg-success text-white h-100">
                <div class="card-body bg-success">
                    <h6 class="text-uppercase">Consume topics</h6>
                    <h1 class="display-4">{{ stat.consumeTopicCount }}</h1>
                </div>
            </div>
        </div>
        <div class="col-lg-4 col-md-6 col-sm-12 py-2">
            <div class="card text-white bg-danger h-100">
                <div class="card-body bg-danger">
                    <h6 class="text-uppercase">Consumers</h6>
                    <h1 class="display-4">{{ stat.consumerCount }}</h1>
                </div>
            </div>
        </div>
        <div class="col-lg-4 col-md-12 col-sm-12 py-2">
            <div class="card text-white bg-info h-100">
                <div class="card-body bg-info">
                    <h6 class="text-uppercase">Failed repeat</h6>
                    <h1 class="display-4">{{ stat.repeatFailedCount }} / {{ stat.repeatAllCount }}</h1>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import DataBusService, { DashboardStatResponse } from "@/services/DataBusService";
import { defineComponent, onUnmounted, ref } from "vue";
import {EventConsumers, EventRepeater, EventRepeaterData, EventConsumersData, useServerEvent} from "@/hooks/useServerEvent";

export default defineComponent({
    setup() {
        const stat = ref({} as DashboardStatResponse)
        DataBusService.getDashboardStat()
            .then((data: DashboardStatResponse) => stat.value = data)
            .catch((e: Error) => console.log(e))

        const close = useServerEvent([
            new EventConsumers((data: EventConsumersData) => {
                stat.value.consumerCount = data.consumerCount
                stat.value.consumeTopicCount = data.consumeTopicCount
            }),
            new EventRepeater((data: EventRepeaterData) => {
                stat.value.repeatAllCount = data.allCount
                stat.value.repeatFailedCount = data.failedCount
            }),
        ])
        onUnmounted(close)

        return { stat }
    }
})
</script>
