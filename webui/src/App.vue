<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  LayoutDashboard,
  Network,
  ShieldCheck,
  ScrollText,
  Settings,
  Sun,
  Moon,
  Monitor,
  LogOut,
  Zap,
  ServerCog,
} from "@lucide/vue-next";
import { useSessionStore } from "./stores/session";
import { api } from "./api/client";
const route = useRoute(),
  router = useRouter(),
  session = useSessionStore();
const mode = ref(localStorage.getItem("ds.theme") || "system");
const showSecret = ref(false),
  username = ref(""),
  password = ref(""),
  setupToken = ref(""),
  setupRequired = ref(false),
  error = ref("");
const isPublic = computed(() => route.meta.public);
const nav = computed(() => [
  ["/", "概览", LayoutDashboard],
  ["/dns", "DNS 解析", Network],
  ["/certificates", "证书", ShieldCheck],
  ["/tasks", "任务日志", ScrollText],
  ["/nodes", "证书节点", ServerCog],
  ["/settings", "设置", Settings],
  ...(session.user?.role === "admin" ? [["/fast-records", "快速解析", Zap] as const] : []),
] as const);
function applyTheme() {
  const dark =
    mode.value === "dark" ||
    (mode.value === "system" &&
      matchMedia("(prefers-color-scheme:dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  localStorage.setItem("ds.theme", mode.value);
}
function cycleTheme() {
  mode.value =
    mode.value === "system"
      ? "light"
      : mode.value === "light"
        ? "dark"
        : "system";
  applyTheme();
}
async function connect() {
  error.value = "";
  try {
    const result: any = await api(
      setupRequired.value ? "/auth/setup" : "/auth/login",
      {
        method: "POST",
        body: JSON.stringify({
          username: username.value,
          password: password.value,
          setupToken: setupToken.value,
        }),
      },
    );
    if (setupRequired.value) {
      setupRequired.value = false;
      error.value = "管理员已创建，请登录";
      return;
    }
    session.establish(result.user, result.csrfToken);
    router.push("/");
  } catch (e) {
    error.value = e instanceof Error ? e.message : "连接失败";
  }
}
async function logout() {
  try {
    await api("/auth/logout", { method: "POST" });
  } finally {
    session.clear();
    router.push("/");
  }
}
onMounted(async () => {
  applyTheme();
  if (!isPublic.value) {
    try {
      const result: any = await api("/auth/session");
      session.establish(result.user, result.csrfToken);
    } catch {
      try {
        const state: any = await api("/auth/setup-status");
        setupRequired.value = state.required;
      } catch {}
    }
  }
});
const themeIcon = computed(() =>
  mode.value === "dark" ? Moon : mode.value === "light" ? Sun : Monitor,
);
</script>
<template>
  <router-view v-if="isPublic" />
  <div v-else-if="!session.connected" class="connect-shell">
    <form class="connect-card" @submit.prevent="connect">
      <div class="brand-mark"><Zap :size="22" /></div>
      <p class="eyebrow">DOMAINSPRITE CONSOLE</p>
      <h1>{{ setupRequired ? "创建首个管理员" : "登录域名控制台" }}</h1>
      <p class="muted">
        {{
          setupRequired
            ? "使用启动日志中的一次性 Setup Token。"
            : "使用平台用户名和密码建立安全会话。"
        }}
      </p>
      <label
        >用户名<input
          v-model="username"
          autocomplete="username"
          required
          placeholder="输入用户名" /></label
      ><label
        >密码
        <div class="input-action">
          <input
            v-model="password"
            :type="showSecret ? 'text' : 'password'"
            autocomplete="current-password"
            required
            placeholder="至少 10 个字符"
          /><button
            type="button"
            class="text-button"
            @click="showSecret = !showSecret"
          >
            {{ showSecret ? "隐藏" : "显示" }}
          </button>
        </div></label
      >
      <label v-if="setupRequired"
        >Setup Token<input v-model="setupToken" type="password" required
      /></label>
      <div v-if="error" class="alert error">{{ error }}</div>
      <button class="primary wide">
        {{ setupRequired ? "创建管理员" : "登录" }}
      </button>
      <p class="security-note">
        密码不会保存在浏览器；登录状态由 HttpOnly Cookie 维护。
      </p>
      <router-link to="/quick" class="quiet-link">前往快速 DDNS</router-link>
      <router-link v-if="!setupRequired" to="/register" class="quiet-link">注册普通用户</router-link>
    </form>
  </div>
  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="logo"><Zap /></div>
      <nav>
        <router-link
          v-for="[to, label, Icon] in nav"
          :key="to"
          :to="to"
          :aria-label="label"
          ><component :is="Icon" /><span>{{ label }}</span></router-link
        >
      </nav>
      <router-link to="/quick" class="quick-link" aria-label="快速 DDNS"
        ><Zap /><span>快速 DDNS</span></router-link
      >
    </aside>
    <main class="main">
      <header class="topbar">
        <div>
          <p class="eyebrow">DOMAINSPRITE</p>
          <h2>{{ route.meta.title }}</h2>
        </div>
        <div class="top-actions">
          <span class="connection"><i></i>服务已连接</span
          ><button
            class="icon-button"
            @click="cycleTheme"
            :title="`主题：${mode}`"
          >
            <component :is="themeIcon" /></button
          ><button class="icon-button" title="退出并清除凭据" @click="logout">
            <LogOut />
          </button>
        </div>
      </header>
      <router-view />
    </main>
  </div>
</template>
