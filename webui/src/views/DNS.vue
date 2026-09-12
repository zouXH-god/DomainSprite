<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { api, ApiError } from "../api/client";
import type { Account, Domain, RecordInfo } from "../api/types";
import { Plus, Pencil, Trash2, X, RefreshCw, Target, SlidersHorizontal } from "@lucide/vue-next";
const accounts = ref<Account[]>([]),
  domains = ref<Domain[]>([]),
  records = ref<RecordInfo[]>([]),
  account = ref(""),
  domain = ref<Domain>(),
  search = ref(""),
  typeFilter = ref(""),
  statusFilter = ref(""),
  lineFilter = ref(""),
  loading = ref(false),
  error = ref(""),
  drawer = ref(false),
  editing = ref<RecordInfo>();
const form = ref<Partial<RecordInfo>>({
  recordType: "A",
  recordName: "@",
  ttl: 600,
  status: "ENABLE",
});
const lower = (value: unknown) => String(value ?? "").toLowerCase();
const typeOptions = computed(() => [...new Set(records.value.map((r) => r.recordType).filter(Boolean))].sort());
const lineOptions = computed(() => [...new Set(records.value.map((r) => r.line || "default"))].sort());
const isEnabled = (status: unknown) => /enable|true/i.test(String(status ?? ""));
const filtered = computed(() => records.value.filter((r) => {
  const matchesSearch = [r?.recordName, r?.recordContent, r?.recordType, r?.line]
    .map(lower).join(" ").includes(lower(search.value));
  const matchesType = !typeFilter.value || r.recordType === typeFilter.value;
  const matchesStatus = !statusFilter.value || (statusFilter.value === "enabled" ? isEnabled(r.status) : !isEnabled(r.status));
  const matchesLine = !lineFilter.value || (r.line || "default") === lineFilter.value;
  return matchesSearch && matchesType && matchesStatus && matchesLine;
}));
const hasFilters = computed(() => Boolean(search.value || typeFilter.value || statusFilter.value || lineFilter.value));
function resetFilters() {
  search.value = "";
  typeFilter.value = "";
  statusFilter.value = "";
  lineFilter.value = "";
}
async function loadAccounts() {
  const result = await api<Account[]>("/api/accounts");
  accounts.value = Array.isArray(result) ? result : [];
  if (accounts.value[0]) account.value = accounts.value[0].name;
}
async function loadDomains() {
  if (!account.value) return;
  domain.value = undefined;
  records.value = [];
  const d: any = await api(
    `/api/${encodeURIComponent(account.value)}/domains?pageNumber=1&pageSize=100`,
  );
  domains.value = Array.isArray(d?.domains) ? d.domains : [];
}
async function loadRecords() {
  if (!domain.value) return;
  loading.value = true;
  try {
    const d: any = await api(
      `/api/${encodeURIComponent(account.value)}/records?domainId=${encodeURIComponent(domain.value.id)}&domainName=${encodeURIComponent(domain.value.domainName)}&pageNumber=1&pageSize=100`,
    );
    records.value = Array.isArray(d?.records) ? d.records : [];
  } catch (e) {
    showError(e);
  } finally {
    loading.value = false;
  }
}
function showError(e: unknown) {
  error.value =
    e instanceof ApiError ? `${e.message} · 请求 ${e.requestId}` : "操作失败";
}
function open(record?: RecordInfo) {
  editing.value = record;
  form.value = record
    ? { ...record }
    : {
        recordType: "A",
        recordName: "@",
        recordContent: "",
        ttl: 600,
        status: "ENABLE",
        line: "default",
        proxied: false,
      };
  drawer.value = true;
}
async function save() {
  if (!domain.value) return;
  const body = {
    ...form.value,
    id: editing.value?.id || "",
    domainId: domain.value.id,
    domainName: domain.value.domainName,
  };
  try {
    await api(`/api/${encodeURIComponent(account.value)}/record`, {
      method: editing.value ? "PUT" : "POST",
      body: JSON.stringify(body),
    });
    drawer.value = false;
    await loadRecords();
  } catch (e) {
    showError(e);
  }
}
async function remove(r: RecordInfo) {
  if (!confirm(`删除 ${r.recordName} → ${r.recordContent}？`)) return;
  try {
    await api(
      `/api/${encodeURIComponent(account.value)}/record?domainName=${encodeURIComponent(domain.value!.domainName)}&recordId=${encodeURIComponent(r.id)}`,
      { method: "DELETE" },
    );
    await loadRecords();
  } catch (e) {
    showError(e);
  }
}
async function toggle(r: RecordInfo) {
  const old = r.status,
    next = /enable|true/i.test(old) ? "DISABLE" : "ENABLE";
  r.status = next;
  try {
    await api(
      `/api/${encodeURIComponent(account.value)}/record/status?domainName=${encodeURIComponent(domain.value!.domainName)}&recordId=${encodeURIComponent(r.id)}&status=${next}`,
      { method: "PUT" },
    );
  } catch (e) {
    r.status = old;
    showError(e);
  }
}
watch(account, loadDomains);
watch(domain, loadRecords);
onMounted(() => loadAccounts().catch(showError));
</script>
<template>
  <div v-if="error" class="alert error" style="margin-bottom: 14px">
    {{ error }}
  </div>
  <div class="toolbar">
    <select v-model="account" aria-label="选择账户">
      <option v-for="a in accounts" :value="a.name">
        {{ a.name }} · {{ a.type }}
      </option></select
    ><select v-model="domain" aria-label="选择域名">
      <option :value="undefined" disabled>选择域名</option>
      <option v-for="d in domains" :value="d">
        {{ d.domainName }}
      </option></select
    ><input v-model="search" placeholder="搜索记录" /><button
      class="icon-button"
      @click="loadRecords"
      title="刷新"
    >
      <RefreshCw /></button
    ><button class="primary" :disabled="!domain" @click="open()">
      <Plus :size="17" /> 新增记录
    </button>
  </div>
  <div class="filter-bar" v-if="domain">
    <span class="filter-label"><SlidersHorizontal />筛选</span>
    <select v-model="typeFilter" aria-label="按记录类型筛选">
      <option value="">全部类型</option>
      <option v-for="type in typeOptions" :key="type" :value="type">{{ type }}</option>
    </select>
    <select v-model="statusFilter" aria-label="按启停状态筛选">
      <option value="">全部状态</option>
      <option value="enabled">已启用</option>
      <option value="disabled">已停用</option>
    </select>
    <select v-model="lineFilter" aria-label="按解析线路筛选">
      <option value="">全部线路</option>
      <option v-for="line in lineOptions" :key="line" :value="line">{{ line }}</option>
    </select>
    <span class="filter-count">显示 {{ filtered.length }} / {{ records.length }} 条</span>
    <button v-if="hasFilters" class="secondary compact-button" @click="resetFilters">清除筛选</button>
  </div>
  <section class="card" v-if="domain">
    <p class="eyebrow">{{ account }} / {{ domain.dnsFrom }}</p>
    <h3 class="mono">{{ domain.domainName }}</h3>
    <p class="muted">Zone ID {{ domain.id }}</p>
  </section>
  <div class="table-wrap" style="margin-top: 16px">
    <table>
      <thead>
        <tr>
          <th>类型</th>
          <th>名称</th>
          <th>内容</th>
          <th>TTL</th>
          <th>线路</th>
          <th>状态</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in filtered" :key="r.id">
          <td>
            <span class="badge">{{ r.recordType }}</span>
          </td>
          <td class="mono">{{ r.recordName }}</td>
          <td class="mono">{{ r.recordContent }}</td>
          <td>{{ r.ttl }}</td>
          <td>{{ r.line || "—" }}</td>
          <td>
            <button
              class="icon-button status-target"
              :class="{ active: /enable|true/i.test(r.status) }"
              :title="/enable|true/i.test(r.status) ? '点击停用解析' : '点击启用解析'"
              :aria-label="/enable|true/i.test(r.status) ? '当前已启用，点击停用' : '当前已停用，点击启用'"
              @click="toggle(r)"
            >
              <Target />
            </button>
          </td>
          <td class="record-actions-cell">
            <div class="row-actions record-actions">
              <button class="icon-button" @click="open(r)" title="编辑">
                <Pencil />
              </button>
              <button class="icon-button danger" @click="remove(r)" title="删除">
                <Trash2 />
              </button>
            </div>
          </td>
        </tr>
        <tr v-if="!filtered.length">
          <td colspan="7" class="empty">
            {{
              loading ? "正在加载…" : domain ? "暂无解析记录" : "请先选择域名"
            }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <div v-if="drawer" class="drawer-backdrop" @click.self="drawer = false">
    <aside class="drawer">
      <div class="drawer-head">
        <div>
          <p class="eyebrow">DNS RECORD</p>
          <h2>{{ editing ? "编辑解析" : "新增解析" }}</h2>
        </div>
        <button class="icon-button" @click="drawer = false"><X /></button>
      </div>
      <form @submit.prevent="save">
        <label
          >记录类型<select v-model="form.recordType">
            <option
              v-for="t in [
                'A',
                'AAAA',
                'CNAME',
                'TXT',
                'MX',
                'NS',
                'SRV',
                'CAA',
              ]"
            >
              {{ t }}
            </option>
          </select></label
        ><label
          >主机记录<input
            v-model="form.recordName"
            required
            placeholder="@ 或 www" /></label
        ><label
          >记录内容<textarea
            v-model="form.recordContent"
            required
            rows="3"
          ></textarea></label
        ><label
          >TTL<input v-model.number="form.ttl" type="number" min="1" /></label
        ><label>线路<input v-model="form.line" /></label
        ><label v-if="domain?.dnsFrom === 'Cloudflare'"
          ><span>Cloudflare 代理</span
          ><input
            v-model="form.proxied"
            type="checkbox"
            style="width: auto" /></label
        ><button class="primary wide">
          {{ editing ? "保存修改" : "创建记录" }}
        </button>
      </form>
    </aside>
  </div>
</template>
