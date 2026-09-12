<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api, ApiError } from "../api/client";
import type { Page, Task } from "../api/types";
import { RefreshCw, Copy, Code2, List } from "@lucide/vue-next";
const route = useRoute(),
  router = useRouter();
const page = ref<Page<Task>>({ items: [], page: 1, pageSize: 30, total: 0 }),
  selected = ref<Task>(),
  raw = ref(""),
  rawMode = ref(false),
  selectedAttempt = ref<number>(),
  filter = ref(String(route.query.state || "")),
  error = ref(""),
  timer = ref<number>();
type LogEvent = Record<string, any> & { _attempt: number; _raw: string };
const events = computed<LogEvent[]>(() => {
  let currentAttempt = 1;
  return raw.value
    .split("\n")
    .filter(Boolean)
    .map((line) => {
      let event: Record<string, any>;
      try {
        event = JSON.parse(line);
      } catch {
        event = { time: "", level: "ERROR", msg: "unparsed", error: line };
      }
      const explicit = Number(event.attempt);
      if (Number.isInteger(explicit) && explicit > 0) currentAttempt = explicit;
      return { ...event, _attempt: currentAttempt, _raw: line } as LogEvent;
    });
});
const attempts = computed(() => {
  const values = new Set(events.value.map((event) => event._attempt));
  if (!values.size && selected.value?.attempt) values.add(selected.value.attempt);
  return [...values].sort((a, b) => a - b);
});
const visibleEvents = computed(() =>
  events.value.filter((event) => event._attempt === selectedAttempt.value),
);
const visibleRaw = computed(() =>
  visibleEvents.value.map((event) => event._raw).join("\n"),
);
function selectLatestAttempt(force = false) {
  const latest = attempts.value.at(-1) || selected.value?.attempt || 1;
  if (force || !selectedAttempt.value || !attempts.value.includes(selectedAttempt.value)) {
    selectedAttempt.value = latest;
  }
}
function fail(e: unknown) {
  error.value =
    e instanceof ApiError ? `${e.message} · ${e.requestId}` : "加载失败";
}
async function load() {
  try {
    page.value = await api(
      `/certificate/tasks?page=1&pageSize=30&state=${filter.value}`,
    );
    const wanted = String(route.params.taskId || "");
    if (wanted) {
      if (selected.value?.taskId !== wanted) selectedAttempt.value = undefined;
      selected.value =
        page.value.items.find((t) => t.taskId === wanted) ||
        (await api(`/certificate/task/${wanted}`));
      await loadLog();
    }
  } catch (e) {
    fail(e);
  }
}
async function choose(t: Task) {
  selected.value = t;
  selectedAttempt.value = undefined;
  router.replace("/tasks/" + t.taskId);
  await loadLog();
}
async function loadLog() {
  if (!selected.value) return;
  try {
    const d: any = await api(`/certificate/task/${selected.value.taskId}/log`);
    selected.value = d.task;
    raw.value = d.log;
    selectLatestAttempt();
    if (!["success", "fail"].includes(d.task.state)) schedule();
  } catch (e) {
    fail(e);
  }
}
function schedule() {
  clearTimeout(timer.value);
  timer.value = window.setTimeout(
    async () => {
      if (document.hidden) {
        schedule();
        return;
      }
      await loadLog();
    },
    document.hidden ? 10000 : 2000,
  );
}
function copyTask() {
  if (selected.value) void navigator.clipboard.writeText(selected.value.taskId);
}
watch(filter, load);
onMounted(load);
onUnmounted(() => clearTimeout(timer.value));
const title = (v: string) =>
  ({
    task_enqueued: "任务入队",
    "task.received": "开始执行",
    "challenge.present_started": "创建验证记录",
    "challenge.present_succeeded": "验证记录已创建",
    "challenge.cleanup_succeeded": "验证记录已清理",
    "challenge.cleanup_deferred": "验证记录将在后台重试清理",
    "acme.obtain_started": "向 CA 申请证书",
    "acme.issued": "证书已签发",
    "staging.recovered": "恢复签发结果",
    "certificate.persisted": "证书已保存",
    "certificate.completed": "证书已激活",
    "task.retry_scheduled": "等待重试",
    "task.failed": "任务失败",
  })[v] || v;
</script>
<template>
  <div v-if="error" class="alert error">{{ error }}</div>
  <div class="toolbar">
    <input
      placeholder="搜索 taskId"
      @keyup.enter="
        router.push('/tasks/' + ($event.target as HTMLInputElement).value);
        load();
      "
    /><select v-model="filter">
      <option value="">全部状态</option>
      <option value="wait">等待</option>
      <option value="apply">处理中</option>
      <option value="success">成功</option>
      <option value="fail">失败</option></select
    ><button class="icon-button" @click="load"><RefreshCw /></button>
  </div>
  <div class="split">
    <section>
      <button
        v-for="t in page.items"
        :key="t.taskId"
        class="list-button"
        :class="{ active: selected?.taskId === t.taskId }"
        @click="choose(t)"
      >
        <span class="badge" :class="t.state">{{ t.state }}</span>
        <p class="mono">{{ t.taskId.slice(0, 18) }}…</p>
        <span class="muted">{{ t.kind }} · 尝试 {{ t.attempt }}</span>
      </button>
      <div v-if="!page.items.length" class="card empty">暂无任务</div>
    </section>
    <section class="card">
      <template v-if="selected"
        ><div class="section-head" style="margin-top: 0">
          <div>
            <p class="eyebrow">TASK DETAIL</p>
            <h3 class="mono">{{ selected.taskId }}</h3>
          </div>
          <div class="toolbar">
            <button
              class="icon-button"
              @click="copyTask"
            >
              <Copy /></button
            ><button class="icon-button" @click="rawMode = !rawMode">
              <component :is="rawMode ? List : Code2" />
            </button>
          </div>
        </div>
        <div class="attempt-switcher" v-if="attempts.length">
          <span class="muted">执行次数</span>
          <button
            v-for="attempt in attempts"
            :key="attempt"
            class="attempt-button"
            :class="{ active: selectedAttempt === attempt }"
            @click="selectedAttempt = attempt"
          >第 {{ attempt }} 次<span v-if="attempt === attempts.at(-1)"> · 最新</span></button>
        </div>
        <pre
          v-if="rawMode"
          class="token-box mono"
          style="white-space: pre-wrap"
          >{{ visibleRaw }}</pre
        >
        <div v-else class="timeline">
          <article
            v-for="(e, i) in visibleEvents"
            :key="`${e._attempt}-${i}`"
            class="event"
            :class="{ 'error-event': e.level === 'ERROR' }"
          >
            <b>{{ title(e.msg) }}</b>
            <p class="muted">
              {{ e.time ? new Date(e.time).toLocaleString() : "" }} ·
              {{ e.stage || "—" }} · 第
              {{ e._attempt }} 次
            </p>
            <p v-if="e.domain" class="mono">
              {{ e.domain }} · {{ e.provider }} / {{ e.account }}
            </p>
            <p v-if="e.error" :class="e.level === 'ERROR' ? 'error' : 'warning-text'">{{ e.error }}</p>
          </article>
          <div v-if="!visibleEvents.length" class="empty">
            日志尚未产生，运行中的任务会自动刷新。
          </div>
        </div></template
      >
      <div v-else class="empty">选择一项任务查看完整日志链</div>
    </section>
  </div>
</template>
