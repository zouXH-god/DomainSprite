<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Pencil, RefreshCw, Search, X, Zap } from "@lucide/vue-next";
import { api, ApiError } from "../api/client";

interface FastRecord {
  id: number;
  providerRecordId: string;
  revision: number;
  domainId: string;
  domainName: string;
  fqdn: string;
  recordName: string;
  recordType: string;
  recordContent: string;
  line: string;
  status: string;
  ttl: number;
  dnsFrom: string;
  updateTime?: string;
}
interface FastRecordPage {
  items: FastRecord[];
  page: number;
  pageSize: number;
  total: number;
}

const records = ref<FastRecord[]>([]);
const page = ref(1), pageSize = 20, total = ref(0);
const search = ref(""), appliedSearch = ref("");
const loading = ref(false), saving = ref(false), error = ref(""), notice = ref("");
const editing = ref<FastRecord>();
const form = ref({ recordName: "", recordContent: "", ttl: 600, revision: 0 });
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)));

function showError(value: unknown) {
  error.value = value instanceof ApiError ? `${value.message} · 请求 ${value.requestId}` : "操作失败";
}
async function load() {
  loading.value = true;
  error.value = "";
  try {
    const result = await api<FastRecordPage>(`/api/fast-ddns/records?page=${page.value}&pageSize=${pageSize}&search=${encodeURIComponent(appliedSearch.value)}`);
    records.value = Array.isArray(result.items) ? result.items : [];
    total.value = result.total || 0;
  } catch (e) {
    showError(e);
  } finally {
    loading.value = false;
  }
}
function submitSearch() {
  appliedSearch.value = search.value.trim();
  page.value = 1;
  load();
}
function open(record: FastRecord) {
  editing.value = record;
  form.value = { recordName: record.recordName, recordContent: record.recordContent, ttl: record.ttl || 600, revision: record.revision };
}
async function save() {
  if (!editing.value || saving.value) return;
  saving.value = true;
  error.value = "";
  try {
    await api(`/api/fast-ddns/records/${encodeURIComponent(editing.value.id)}`, {
      method: "PUT",
      body: JSON.stringify(form.value),
    });
    editing.value = undefined;
    notice.value = "快速解析记录已更新";
    window.setTimeout(() => notice.value = "", 1800);
    await load();
  } catch (e) {
    showError(e);
  } finally {
    saving.value = false;
  }
}
async function changePage(next: number) {
  if (next < 1 || next > pageCount.value || next === page.value) return;
  page.value = next;
  await load();
}
onMounted(load);
</script>

<template>
  <div v-if="error" class="alert error">{{ error }}</div>
  <div v-if="notice" class="alert success">{{ notice }}</div>
  <section class="card">
    <div class="settings-head">
      <div>
        <h3><Zap />快速解析记录</h3>
        <p class="muted">查看并修改已创建的快速 DDNS A 记录。更新 token 不会在管理端显示。</p>
      </div>
      <button class="secondary" :disabled="loading" @click="load"><RefreshCw :class="{ spinning: loading }" />刷新</button>
    </div>
    <form class="toolbar fast-search" @submit.prevent="submitSearch">
      <input v-model="search" aria-label="搜索快速解析" placeholder="搜索域名、IP、厂商…">
      <button class="primary" type="submit"><Search />搜索</button>
    </form>
    <div class="table-wrap">
      <table>
        <thead><tr><th>域名</th><th>类型</th><th>记录值</th><th>TTL</th><th>线路 / 状态</th><th>厂商</th><th></th></tr></thead>
        <tbody>
          <tr v-for="record in records" :key="record.id">
            <td><b class="mono">{{ record.fqdn }}</b><br><small class="muted mono">Record ID {{ record.providerRecordId }}</small></td>
            <td><span class="badge">{{ record.recordType || "A" }}</span></td>
            <td class="mono">{{ record.recordContent }}</td>
            <td>{{ record.ttl || "默认" }}</td>
            <td>{{ record.line || "default" }} · {{ record.status || "—" }}</td>
            <td>{{ record.dnsFrom || "—" }}</td>
            <td class="record-actions-cell"><button class="icon-button" title="编辑快速解析" @click="open(record)"><Pencil /></button></td>
          </tr>
        </tbody>
      </table>
      <div v-if="!loading && !records.length" class="empty">{{ appliedSearch ? "没有匹配的快速解析记录" : "暂无快速解析记录" }}</div>
    </div>
    <div class="fast-pagination">
      <span class="muted">共 {{ total }} 条 · 第 {{ page }} / {{ pageCount }} 页</span>
      <span class="row-actions"><button class="secondary" :disabled="page <= 1" @click="changePage(page - 1)">上一页</button><button class="secondary" :disabled="page >= pageCount" @click="changePage(page + 1)">下一页</button></span>
    </div>
  </section>

  <div v-if="editing" class="modal-backdrop" @click.self="editing=undefined">
    <section class="modal-card">
      <div class="drawer-head"><div><p class="eyebrow">QUICK DDNS</p><h2>编辑快速解析</h2><p class="mono muted">{{ editing.fqdn }}</p></div><button class="icon-button" @click="editing=undefined"><X /></button></div>
      <form @submit.prevent="save">
        <label>记录名<input v-model.trim="form.recordName" required maxlength="253" placeholder="例如 host001"></label>
        <label>IPv4 地址<input v-model.trim="form.recordContent" required inputmode="decimal" placeholder="例如 203.0.113.10"></label>
        <label>TTL（秒）<input v-model.number="form.ttl" type="number" min="1" max="86400" required></label>
        <div class="selection-summary"><small>实际域名</small><b class="mono">{{ form.recordName === '@' ? editing.domainName : `${form.recordName}.${editing.domainName}` }}</b><small>固定类型</small><b>A 记录</b></div>
        <button class="primary wide" :disabled="saving">{{ saving ? "保存中…" : "保存修改" }}</button>
      </form>
    </section>
  </div>
</template>
