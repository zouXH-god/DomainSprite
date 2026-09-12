<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, ApiError } from "../api/client";
import type { Task } from "../api/types";
import { ArrowRight, Plus, RefreshCw, TriangleAlert } from "@lucide/vue-next";
const router = useRouter();
interface DashboardData {
  accounts: number;
  providerDistribution: Record<string, number>;
  domains: number;
  certificates: { total: number; success: number; pending: number; failed: number; expiring: number };
  tasks: number;
  recentTasks: Task[];
  health: { database: string; redis: string; worker: string };
}
const emptyDashboard = (): DashboardData => ({
  accounts: 0,
  providerDistribution: {},
  domains: 0,
  certificates: { total: 0, success: 0, pending: 0, failed: 0, expiring: 0 },
  tasks: 0,
  recentTasks: [],
  health: { database: "unknown", redis: "unknown", worker: "unknown" },
});
const data = ref<DashboardData>(emptyDashboard());
const loading = ref(true);
const error = ref("");
async function load() {
  error.value = "";
  loading.value = true;
  try {
    const result = await api<DashboardData>("/api/dashboard");
    data.value = {
      ...emptyDashboard(),
      ...result,
      certificates: { ...emptyDashboard().certificates, ...(result?.certificates || {}) },
      health: { ...emptyDashboard().health, ...(result?.health || {}) },
      recentTasks: result?.recentTasks || [],
      providerDistribution: result?.providerDistribution || {},
    };
  } catch (e) {
    error.value =
      e instanceof ApiError ? `${e.message} · ${e.requestId}` : "加载失败";
  } finally {
    loading.value = false;
  }
}
onMounted(load);
const stageName = (v: string) =>
  ({ wait: "等待", apply: "处理中", success: "成功", fail: "失败" })[v] || v;
</script>
<template>
  <div v-if="error" class="alert error">{{ error }}</div>
  <div class="grid stats" :aria-busy="loading">
    <div class="card">
      <p class="eyebrow">DNS ACCOUNTS</p>
      <div class="stat-value">{{ data.accounts }}</div>
      <span class="muted">已连接账户</span>
      <p class="mono muted provider-breakdown">
        {{ Object.entries(data.providerDistribution || {}).map(([provider, count]) => `${provider} ${count}`).join(" · ") || "暂无厂商" }}
      </p>
    </div>
    <div class="card">
      <p class="eyebrow">DOMAINS</p>
      <div class="stat-value">{{ data.domains }}</div>
      <span class="muted">已同步域名</span>
    </div>
    <div class="card">
      <p class="eyebrow">CERTIFICATES</p>
      <div class="stat-value">{{ data.certificates.success }}</div>
      <span class="muted">有效证书</span>
    </div>
    <div class="card">
      <p class="eyebrow">ATTENTION</p>
      <div class="stat-value">
        {{ data.certificates.failed + data.certificates.expiring }}
      </div>
      <span class="muted">失败或临期</span>
    </div>
  </div>
  <div class="section-head">
    <h3>快捷操作</h3>
    <button class="icon-button" @click="load" title="刷新">
      <RefreshCw />
    </button>
  </div>
  <div class="grid stats">
    <button class="card list-button" @click="router.push('/dns')">
      <Plus />
      <h3>新增解析</h3>
      <span class="muted">从账户和域名开始</span></button
    ><button
      class="card list-button"
      @click="router.push('/certificates?apply=1')"
    >
      <Plus />
      <h3>申请证书</h3>
      <span class="muted">支持跨账户多域名</span></button
    ><button class="card list-button" @click="router.push('/tasks?state=fail')">
      <TriangleAlert />
      <h3>失败任务</h3>
      <span class="muted">定位完整日志链</span>
    </button>
    <div class="card">
      <p class="eyebrow">SERVICE HEALTH</p>
      <h3>{{ data.health.redis === "ok" ? "运行正常" : "服务降级" }}</h3>
      <span class="muted"
        >数据库 {{ data.health.database }} · Redis {{ data.health.redis }}</span
      >
    </div>
  </div>
  <div class="section-head">
    <h3>最近任务</h3>
    <button class="secondary" @click="router.push('/tasks')">
      查看全部 <ArrowRight :size="16" />
    </button>
  </div>
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>任务</th>
          <th>类型</th>
          <th>状态</th>
          <th>尝试</th>
          <th>创建时间</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="t in data.recentTasks"
          :key="t.taskId"
          @click="router.push('/tasks/' + t.taskId)"
        >
          <td class="mono">{{ t.taskId.slice(0, 12) }}…</td>
          <td>{{ t.kind === "renew" ? "续期" : "签发" }}</td>
          <td>
            <span class="badge" :class="t.state">{{ stageName(t.state) }}</span>
          </td>
          <td>{{ t.attempt }}</td>
          <td>{{ new Date(t.createTime).toLocaleString() }}</td>
        </tr>
        <tr v-if="!data.recentTasks.length">
          <td colspan="5" class="empty">暂无任务</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
