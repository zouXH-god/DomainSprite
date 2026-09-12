<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { api, download, ApiError } from "../api/client";
import type { Account, Certificate, Domain, Page } from "../api/types";
import {
  Plus,
  Download,
  RefreshCw,
  X,
  ArrowRight,
  ShieldCheck,
  Eye,
  Copy,
  Trash2,
} from "@lucide/vue-next";
const router = useRouter(),
  route = useRoute();
const page = ref<Page<Certificate>>({
    items: [],
    page: 1,
    pageSize: 20,
    total: 0,
  }),
  error = ref(""),
  wizard = ref(false),
  step = ref(1),
  mode = ref<"managed" | "external">("managed"),
  accounts = ref<Account[]>([]),
  selectedAccount = ref(""),
  domains = ref<Domain[]>([]),
  selectedDomain = ref(""),
  chosen = ref<
    Array<{
      accountName: string;
      domainId: string;
      domainName: string;
      type: string;
    }>
  >([]),
  externalText = ref(""),
  checks = ref<any[]>([]),
  checkingDomains = ref(new Set<string>()),
  checkingAll = ref(false),
  lastCheckedAt = ref<Date>(),
  viewerOpen = ref(false),
  viewerLoading = ref(false),
  viewerType = ref<"cert" | "key">("cert"),
  viewerCertificate = ref<Certificate>(),
  viewerContent = ref(""),
  copied = ref(false),
  submitting = ref(false);
const externalDomains = computed(() => [
  ...new Set(
    String(externalText.value ?? "")
      .split(/[\s,]+/)
      .map((v) => String(v ?? "").trim().toLowerCase())
      .filter(Boolean),
  ),
]);
const sans = computed(() => {
  const names =
    mode.value === "managed"
      ? chosen.value.map((v) => v.domainName)
      : externalDomains.value;
  return names.flatMap((v) => [v, `*.${v}`]);
});
function fail(e: unknown) {
  error.value =
    e instanceof ApiError ? `${e.message} · ${e.requestId}` : "操作失败";
}
async function load() {
  try {
    page.value = await api("/certificate/page?page=1&pageSize=50");
  } catch (e) {
    fail(e);
  }
}
async function initWizard() {
  wizard.value = true;
  step.value = 1;
  checks.value = [];
  try {
    accounts.value = await api("/api/accounts");
    selectedAccount.value = accounts.value[0]?.name || "";
  } catch (e) {
    fail(e);
  }
}
async function loadDomains() {
  if (!selectedAccount.value) return;
  const d: any = await api(
    `/api/${encodeURIComponent(selectedAccount.value)}/domains?pageNumber=1&pageSize=100`,
  );
  domains.value = d.domains || [];
}
function addDomain() {
  const d = domains.value.find((v) => v.id === selectedDomain.value);
  if (
    d &&
    !chosen.value.some(
      (v) => v.accountName === selectedAccount.value && v.domainId === d.id,
    )
  )
    chosen.value.push({
      accountName: selectedAccount.value,
      domainId: d.id,
      domainName: d.domainName,
      type: d.dnsFrom,
    });
}
async function next() {
  if (step.value === 1) {
    if (mode.value === "managed" && !chosen.value.length) return;
    if (mode.value === "external" && !externalDomains.value.length) return;
    step.value = 2;
  } else if (step.value === 2 && mode.value === "external") {
    try {
      if (await checkAllCNAME()) step.value = 3;
    } catch (e) {
      fail(e);
    }
  } else step.value++;
}
async function requestCNAMECheck(domainsToCheck: string[]) {
  return await api<any[]>("/certificate/apply/check-cname", {
    method: "POST",
    body: JSON.stringify({ domains: domainsToCheck }),
  });
}
async function checkAllCNAME() {
  if (!externalDomains.value.length || checkingAll.value) return false;
  checkingAll.value = true;
  error.value = "";
  try {
    const result = await requestCNAMECheck(externalDomains.value);
    checks.value = Array.isArray(result) ? result : [];
    lastCheckedAt.value = new Date();
    return true;
  } catch (e) {
    fail(e);
    return false;
  } finally {
    checkingAll.value = false;
  }
}
async function checkOneCNAME(item: any) {
  const domainName = String(item?.domain || "");
  if (!domainName || checkingDomains.value.has(domainName)) return;
  checkingDomains.value = new Set(checkingDomains.value).add(domainName);
  error.value = "";
  try {
    const result = await requestCNAMECheck([domainName]);
    const refreshed = result?.[0];
    if (refreshed) {
      const index = checks.value.findIndex((value) => value.domain === domainName);
      if (index >= 0) checks.value.splice(index, 1, refreshed);
    }
    lastCheckedAt.value = new Date();
  } catch (e) {
    fail(e);
  } finally {
    const next = new Set(checkingDomains.value);
    next.delete(domainName);
    checkingDomains.value = next;
  }
}
const allCNAMEValid = computed(
  () => checks.value.length === externalDomains.value.length && checks.value.length > 0 && checks.value.every((item) => item.valid),
);
async function submit() {
  submitting.value = true;
  try {
    let result: any;
    if (mode.value === "managed")
      result = await api("/certificate/apply/accounts", {
        method: "POST",
        body: JSON.stringify({
          domains: chosen.value.map(({ accountName, domainId }) => ({
            accountName,
            domainId,
          })),
        }),
      });
    else
      result = await api("/certificate/apply", {
        method: "POST",
        body: JSON.stringify({
          domainNameList: externalDomains.value.join(","),
        }),
      });
    wizard.value = false;
    router.push("/tasks/" + result.taskId);
  } catch (e) {
    fail(e);
  } finally {
    submitting.value = false;
  }
}
async function renew(c: Certificate) {
  if (
    !confirm(
      `为 ${c.commonName || "#" + c.id} 创建新的续期版本？旧证书将在成功前保持生效。`,
    )
  )
    return;
  try {
    const d: any = await api(`/certificate/${c.id}/renew`, { method: "POST" });
    router.push("/tasks/" + d.taskId);
  } catch (e) {
    fail(e);
  }
}
async function viewCertificate(c: Certificate, type: "cert" | "key" = "cert") {
  viewerOpen.value = true;
  viewerLoading.value = true;
  viewerCertificate.value = c;
  viewerType.value = type;
  viewerContent.value = "";
  copied.value = false;
  try {
    const result = await api<{ content: string }>(`/certificate/${c.id}/content?type=${type}`);
    viewerContent.value = result.content || "";
  } catch (e) { fail(e); }
  finally { viewerLoading.value = false; }
}
async function copyCertificate() {
  if (!viewerContent.value) return;
  await navigator.clipboard.writeText(viewerContent.value);
  copied.value = true;
  window.setTimeout(() => (copied.value = false), 1600);
}
async function removeCertificate(c: Certificate) {
  const name = c.commonName || `证书 #${c.id}`;
  if (!confirm(`确定删除 ${name}？\n\n证书文件、任务日志关联和数据库记录将被删除。当前被域名使用或仍在运行的证书不会被允许删除。`)) return;
  try {
    await api(`/certificate/${c.id}`, { method: "DELETE" });
    await load();
  } catch (e) { fail(e); }
}
function days(v: string) {
  return v ? Math.ceil((new Date(v).getTime() - Date.now()) / 86400000) : 0;
}
watch(selectedAccount, () => loadDomains().catch(fail));
onMounted(() => {
  load();
  if (route.query.apply) initWizard();
});
</script>
<template>
  <div v-if="error" class="alert error" style="margin-bottom: 14px">
    {{ error }}
  </div>
  <div class="toolbar">
    <button class="primary" @click="initWizard">
      <Plus :size="17" /> 申请证书</button
    ><button class="icon-button" @click="load"><RefreshCw /></button>
  </div>
  <div
    class="grid"
    style="grid-template-columns: repeat(auto-fill, minmax(330px, 1fr))"
  >
    <article class="card" v-for="c in page.items" :key="c.id">
      <div style="display: flex; justify-content: space-between">
        <span class="badge" :class="c.stage">{{ c.stage }}</span
        ><span class="mono muted">#{{ c.id }}</span>
      </div>
      <h3 class="mono" style="font-size: 18px; margin-top: 18px">
        {{ c.commonName || "等待签发" }}
      </h3>
      <p class="muted">{{ c.domainList || "域名信息处理中" }}</p>
      <div class="steps">
        <span class="step" :class="{ active: c.stage === 'wait' }">排队</span
        ><span class="step" :class="{ active: c.stage === 'challenging' }"
          >验证</span
        ><span
          class="step"
          :class="{ active: ['issued', 'persisted'].includes(c.stage) }"
          >保存</span
        ><span class="step" :class="{ active: c.stage === 'success' }"
          >完成</span
        >
      </div>
      <p v-if="c.notAfter">
        <b>{{ days(c.notAfter) }}</b> 天后到期
      </p>
      <div class="toolbar">
        <button class="secondary" @click="router.push('/tasks/' + c.taskId)">
          任务 <ArrowRight :size="15" /></button
        ><button
          class="secondary"
          :disabled="c.stage !== 'success'"
          @click="renew(c)"
        >
          续期</button
        ><button
          class="icon-button"
          :disabled="c.stage !== 'success'"
          @click="viewCertificate(c)"
          title="在线查看证书"
        >
          <Eye />
        </button><button
          class="icon-button"
          :disabled="c.stage !== 'success'"
          @click="
            download(
              `/certificate/download?certificateId=${c.id}&downloadType=all`,
            )
          "
          title="下载"
        >
          <Download />
        </button><button
          class="icon-button danger"
          @click="removeCertificate(c)"
          title="删除证书"
        >
          <Trash2 />
        </button>
      </div>
    </article>
    <div v-if="!page.items.length" class="card empty">暂无证书</div>
  </div>
  <div v-if="viewerOpen" class="drawer-backdrop" @click.self="viewerOpen = false">
    <aside class="drawer certificate-viewer">
      <div class="drawer-head">
        <div><p class="eyebrow">CERTIFICATE #{{ viewerCertificate?.id }}</p><h2>在线查看</h2></div>
        <button class="icon-button" @click="viewerOpen = false"><X /></button>
      </div>
      <div class="toolbar">
        <button :class="viewerType === 'cert' ? 'primary' : 'secondary'" @click="viewerCertificate && viewCertificate(viewerCertificate, 'cert')">证书 PEM</button>
        <button :class="viewerType === 'key' ? 'primary' : 'secondary'" @click="viewerCertificate && viewCertificate(viewerCertificate, 'key')">私钥 PEM</button>
        <button class="secondary" :disabled="viewerLoading || !viewerContent" @click="copyCertificate"><Copy />{{ copied ? "已复制" : "复制内容" }}</button>
      </div>
      <div v-if="viewerType === 'key'" class="alert warning">私钥属于敏感内容，请勿粘贴到聊天、工单或公开网站。</div>
      <div v-if="viewerLoading" class="empty">正在读取…</div>
      <pre v-else class="certificate-content mono">{{ viewerContent }}</pre>
    </aside>
  </div>
  <div v-if="wizard" class="drawer-backdrop" @click.self="wizard = false">
    <aside class="drawer">
      <div class="drawer-head">
        <div>
          <p class="eyebrow">
            CERTIFICATE WIZARD · {{ step }} / {{ mode === "external" ? 3 : 2 }}
          </p>
          <h2>申请通配符证书</h2>
        </div>
        <button class="icon-button" @click="wizard = false"><X /></button>
      </div>
      <template v-if="step === 1"
        ><div class="toolbar">
          <button
            :class="mode === 'managed' ? 'primary' : 'secondary'"
            @click="mode = 'managed'"
          >
            已管理域名</button
          ><button
            :class="mode === 'external' ? 'primary' : 'secondary'"
            @click="mode = 'external'"
          >
            外部域名
          </button>
        </div>
        <div v-if="mode === 'managed'" class="grid">
          <label
            >账户<select v-model="selectedAccount">
              <option v-for="a in accounts" :value="a.name">
                {{ a.name }} · {{ a.type }}
              </option>
            </select></label
          ><label
            >域名<select v-model="selectedDomain">
              <option value="" disabled>选择域名</option>
              <option v-for="d in domains" :value="d.id">
                {{ d.domainName }}
              </option>
            </select></label
          ><button class="secondary" @click="addDomain">加入证书</button
          ><button
            v-for="(d, i) in chosen"
            class="list-button"
            @click="chosen.splice(i, 1)"
          >
            <b class="mono">{{ d.domainName }}</b
            ><br /><span class="muted">{{ d.accountName }} · {{ d.type }}</span>
          </button>
        </div>
        <label v-else
          >基础域名<textarea
            v-model="externalText"
            rows="8"
            placeholder="example.com&#10;example.org"
          ></textarea
          ><span>{{ externalDomains.length }} / 50 个域名</span></label
        ></template
      ><template v-else-if="step === 2"
        ><ShieldCheck />
        <h3>确认证书名称</h3>
        <p class="muted">将申请 {{ sans.length }} 个 SAN。</p>
        <div class="token-box mono" v-for="s in sans">{{ s }}</div></template
      ><template v-else
        ><div class="section-head" style="margin-top: 0">
          <div><h3>CNAME 委托检查</h3><p class="muted" v-if="lastCheckedAt">最后检查：{{ lastCheckedAt.toLocaleTimeString() }}</p></div>
          <button class="secondary" :disabled="checkingAll" @click="checkAllCNAME"><RefreshCw :class="{ spinning: checkingAll }" />{{ checkingAll ? "检查中…" : "全部重新检查" }}</button>
        </div>
        <article class="card" v-for="c in checks">
          <div class="section-head" style="margin: 0"><span class="badge" :class="c.valid ? 'success' : 'fail'">{{ c.valid ? "已生效" : "未生效" }}</span><button class="icon-button" title="重新检查此域名" :disabled="checkingDomains.has(c.domain)" @click="checkOneCNAME(c)"><RefreshCw :class="{ spinning: checkingDomains.has(c.domain) }" /></button></div>
          <p class="mono">{{ c.fullDomainName }}</p>
          <p class="muted mono">CNAME → {{ c.value }}</p>
        </article>
        <div v-if="allCNAMEValid" class="alert success">全部 CNAME 已生效，可以提交证书申请。</div>
        <div v-else class="alert warning">请在 DNS 生效后重新检查；只有全部通过才能提交。</div></template
      >
      <div class="toolbar" style="margin-top: 24px">
        <button class="secondary" v-if="step > 1" @click="step--">上一步</button
        ><button
          class="primary"
          v-if="step < (mode === 'external' ? 3 : 2)"
          @click="next"
        >
          下一步</button
        ><button
          class="primary"
          v-else
          :disabled="
            submitting || checkingAll || (mode === 'external' && !allCNAMEValid)
          "
          @click="submit"
        >
          {{ submitting ? "正在提交…" : "提交申请" }}
        </button>
      </div>
    </aside>
  </div>
</template>
