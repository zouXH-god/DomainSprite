<script setup lang="ts">
import { ref } from "vue";
import { api, ApiError } from "../api/client";
import { Zap, Copy, ArrowLeft } from "@lucide/vue-next";
const salt = ref(sessionStorage.getItem("ds.fastSalt") || ""),
  remember = ref(!!sessionStorage.getItem("ds.fastSalt")),
  token = ref(""),
  created = ref<any>(),
  updated = ref<any>(),
  error = ref("");
async function create() {
  error.value = "";
  try {
    if (remember.value) sessionStorage.setItem("ds.fastSalt", salt.value);
    else sessionStorage.removeItem("ds.fastSalt");
    created.value = await api("/fast/ip2a", {
      method: "POST",
      headers: { AccessSalt: salt.value },
    });
    token.value = created.value.token;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "创建失败";
  }
}
async function update() {
  error.value = "";
  try {
    updated.value = await api("/fast/record", {
      method: "PUT",
      body: JSON.stringify({ token: token.value }),
    });
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "更新失败";
  }
}
function saveToken() {
  const blob = new Blob([token.value], { type: "text/plain" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = "domainsprite-token.txt";
  a.click();
  URL.revokeObjectURL(a.href);
}
function copyToken() {
  if (created.value?.token) void navigator.clipboard.writeText(created.value.token);
}
</script>
<template>
  <main class="quick-page">
    <div class="quick-inner">
      <router-link to="/" class="quiet-link"
        ><ArrowLeft :size="16" /> 管理控制台</router-link
      >
      <div style="margin-top: 34px">
        <div class="brand-mark"><Zap /></div>
        <p class="eyebrow" style="margin-top: 18px">QUICK DDNS</p>
        <h1 style="font-size: 38px; letter-spacing: -0.05em; margin: 4px 0">
          让地址始终指向这里。
        </h1>
        <p class="muted">
          创建一个动态解析，或使用已有 token 更新当前来源 IP。
        </p>
      </div>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div class="grid quick-grid">
        <section class="card">
          <h2>创建解析</h2>
          <p class="muted">
            AccessSalt 只用于本次创建，不会与管理后台凭据共享。
          </p>
          <label
            >AccessSalt<input
              v-model="salt"
              type="password"
              autocomplete="off" /></label
          ><label
            style="
              display: flex;
              grid-template-columns: auto 1fr;
              margin: 14px 0;
            "
            ><input
              type="checkbox"
              v-model="remember"
              style="width: auto"
            />仅在当前会话保存</label
          ><button class="primary wide" @click="create">
            创建并获取 Token
          </button>
          <div v-if="created" style="margin-top: 18px">
            <p class="mono">
              <b
                >{{ created.recordInfo.recordName }}.{{
                  created.recordInfo.domainName
                }}</b
              >
              → {{ created.recordInfo.recordContent }}
            </p>
            <div class="token-box mono">{{ created.token }}</div>
            <div class="toolbar" style="margin-top: 10px">
              <button
                class="secondary"
                @click="copyToken"
              >
                <Copy :size="16" /> 复制</button
              ><button class="secondary" @click="saveToken">下载文本</button>
            </div>
          </div>
        </section>
        <section class="card">
          <h2>更新解析</h2>
          <p class="muted">服务会使用当前请求来源 IP 更新 token 对应的记录。</p>
          <label
            >更新 Token<textarea
              v-model="token"
              rows="5"
              placeholder="粘贴创建时获得的 token"
            ></textarea></label
          ><button
            class="primary wide"
            style="margin-top: 14px"
            @click="update"
          >
            更新当前 IP
          </button>
          <div v-if="updated" class="alert success" style="margin-top: 18px">
            <b>更新完成</b><br /><span class="mono"
              >{{ updated.recordInfo.recordName }} →
              {{ updated.recordInfo.recordContent }}</span
            >
          </div>
        </section>
      </div>
    </div>
  </main>
</template>
