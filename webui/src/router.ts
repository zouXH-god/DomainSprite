import { createRouter, createWebHistory } from "vue-router";
import Dashboard from "./views/Dashboard.vue";
import DNS from "./views/DNS.vue";
import Certificates from "./views/Certificates.vue";
import Tasks from "./views/Tasks.vue";
import Settings from "./views/Settings.vue";
import Quick from "./views/Quick.vue";
import Register from "./views/Register.vue";
import Nodes from "./views/Nodes.vue";
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: Dashboard, meta: { title: "概览" } },
    { path: "/dns", component: DNS, meta: { title: "DNS 解析" } },
    { path: "/certificates", component: Certificates, meta: { title: "证书" } },
    { path: "/tasks/:taskId?", component: Tasks, meta: { title: "任务日志" } },
    { path: "/settings", component: Settings, meta: { title: "设置" } },
    { path: "/nodes", component: Nodes, meta: { title: "证书节点" } },
    {
      path: "/quick",
      component: Quick,
      meta: { public: true, title: "快速 DDNS" },
    },
    { path: "/register", component: Register, meta: { public: true, title: "注册" } },
  ],
});
