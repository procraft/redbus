import {createWebHistory, createRouter} from "vue-router";
import {RouteRecordRaw} from "vue-router";
import Dashboard from "@/pages/Dashboard.vue";
import FailedRepeat from "@/pages/FailedRepeat.vue";
import TopicStat from "@/pages/TopicStat.vue";

export enum pages {
    Dashboard = 'Dashboard',
    FailedRepeat = 'FailedRepeat',
    TopicStat = 'TopicStat',
}

const routes: Array<RouteRecordRaw> = [
    {
        path: "/",
        alias: "/dashboard",
        name: pages.Dashboard,
        component: Dashboard,
    },
    {
        path: "/",
        alias: "/topic-stat",
        name: pages.TopicStat,
        component: TopicStat,
    },
    {
        path: "/",
        alias: "/failed-repeat",
        name: pages.FailedRepeat,
        component: FailedRepeat,
    },
];

const router = createRouter({
    history: createWebHistory(),
    routes,
});

export default router;
