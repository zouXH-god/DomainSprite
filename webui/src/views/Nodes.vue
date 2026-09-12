<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "../api/client";
import { ServerCog, Plus, Copy, Pause, Play, Ban, Pencil, X, Layers3 } from "@lucide/vue-next";
import { useSessionStore } from "../stores/session";

const session = useSessionStore();
const nodes = ref<any[]>([]), groups = ref<any[]>([]), error = ref("");
const selectedGroupId = ref(0), groupModal = ref(false), registrationModal = ref(false), saving = ref(false);
const editingGroupId = ref(0), tokenResult = ref<any>(null);
const groupForm = ref({ name: "", description: "", enabled: true, isDefault: false });
const tokenForm = ref({ groupId: 0, expiresMinutes: 10 });
const selectedGroup = computed(() => groups.value.find(g => g.id === selectedGroupId.value));
const visibleNodes = computed(() => nodes.value.filter(n => n.groupId === selectedGroupId.value));
const onlineInGroup = computed(() => visibleNodes.value.filter(n => n.status === "online").length);

function fail(e: unknown) { error.value = e instanceof ApiError ? `${e.message} · ${e.requestId}` : "操作失败"; }
async function load() {
  try {
    const [nodeRows, groupRows] = await Promise.all([api<any[]>("/api/nodes"), api<any[]>("/api/node-groups")]);
    nodes.value = Array.isArray(nodeRows) ? nodeRows : [];
    groups.value = Array.isArray(groupRows) ? groupRows : [];
    if (!groups.value.some(g => g.id === selectedGroupId.value)) selectedGroupId.value = groups.value.find(g => g.isDefault)?.id || groups.value[0]?.id || 0;
  } catch (e) { fail(e); }
}
function openCreateGroup() { editingGroupId.value = 0; groupForm.value = { name: "", description: "", enabled: true, isDefault: false }; groupModal.value = true; }
function openEditGroup(group: any) { editingGroupId.value = group.id; groupForm.value = { name: group.name, description: group.description || "", enabled: group.enabled, isDefault: group.isDefault }; groupModal.value = true; }
async function saveGroup() {
  if (!groupForm.value.name.trim() || saving.value) return;
  saving.value = true;
  try {
    const saved: any = await api(editingGroupId.value ? `/api/node-groups/${editingGroupId.value}` : "/api/node-groups", { method: editingGroupId.value ? "PUT" : "POST", body: JSON.stringify(groupForm.value) });
    groupModal.value = false; selectedGroupId.value = saved.id; await load();
  } catch (e) { fail(e); } finally { saving.value = false; }
}
function openRegistration() { if (!selectedGroupId.value) return; tokenForm.value = { groupId: selectedGroupId.value, expiresMinutes: 10 }; tokenResult.value = null; registrationModal.value = true; }
async function createToken() {
  if (saving.value) return; saving.value = true;
  try { tokenResult.value = await api("/api/nodes/registration-tokens", { method: "POST", body: JSON.stringify(tokenForm.value) }); }
  catch (e) { fail(e); } finally { saving.value = false; }
}
async function action(id: number, name: string) { try { await api(`/api/nodes/${id}/${name}`, { method: "POST" }); await load(); } catch (e) { fail(e); } }
async function copy(value: string) { await navigator.clipboard.writeText(value); }
onMounted(load);
</script>

<template>
  <div v-if="error" class="alert error">{{ error }}</div>
  <section class="card node-workspace">
    <aside class="node-groups">
      <div class="node-pane-head"><div><p class="eyebrow">NODE GROUPS</p><h3>节点分组</h3></div><button v-if="session.user?.role === 'admin'" class="icon-button" title="创建分组" @click="openCreateGroup"><Plus /></button></div>
      <div v-for="group in groups" :key="group.id" class="group-item" :class="{ active: selectedGroupId === group.id }" role="button" tabindex="0" @click="selectedGroupId = group.id" @keydown.enter="selectedGroupId = group.id">
        <span class="group-icon"><Layers3 /></span><span><b>{{ group.name }}</b><small>{{ nodes.filter(n => n.groupId === group.id).length }} 个节点<span v-if="group.isDefault"> · 默认</span></small></span>
        <button v-if="session.user?.role === 'admin'" class="group-edit" title="编辑分组" @click.stop="openEditGroup(group)"><Pencil /></button>
      </div>
      <div v-if="!groups.length" class="empty compact">尚未创建节点组</div>
    </aside>
    <div class="node-list-pane">
      <div class="node-pane-head"><div><p class="eyebrow">CERTIFICATE NODES</p><h2>{{ selectedGroup?.name || "选择节点组" }}</h2><p v-if="selectedGroup" class="muted">{{ selectedGroup.description || "暂无分组说明" }} · 在线 {{ onlineInGroup }}/{{ visibleNodes.length }}</p></div><button v-if="session.user?.role === 'admin'" class="primary" :disabled="!selectedGroupId" @click="openRegistration"><Plus />注册节点</button></div>
      <div v-if="!visibleNodes.length" class="empty"><ServerCog /><p>{{ selectedGroupId ? "当前分组尚未注册节点" : "请先创建或选择节点组" }}</p></div>
      <article v-for="node in visibleNodes" :key="node.id" class="node-card">
        <div><div class="node-title"><i class="status-dot" :class="node.status"></i><b>{{ node.name }}</b><span class="badge">{{ node.status }}</span></div><p class="muted">{{ node.remark || "无备注" }}</p><small class="mono">{{ node.version || "版本未上报" }} · {{ node.lastHeartbeatAt || "无心跳" }}</small></div>
        <div class="node-load"><span>当前负载</span><b>{{ node.running }}/{{ node.capacity }}</b></div>
        <div v-if="session.user?.role === 'admin'" class="row-actions"><button class="icon-button" title="暂停调度" @click="action(node.id, 'drain')"><Pause /></button><button class="icon-button" title="恢复调度" @click="action(node.id, 'resume')"><Play /></button><button class="icon-button danger" title="撤销节点身份" @click="action(node.id, 'revoke')"><Ban /></button></div>
      </article>
    </div>
  </section>

  <div v-if="groupModal" class="modal-backdrop" @click.self="groupModal = false"><section class="modal-card"><div class="drawer-head"><div><p class="eyebrow">NODE GROUP</p><h2>{{ editingGroupId ? "编辑分组" : "创建分组" }}</h2></div><button class="icon-button" @click="groupModal = false"><X /></button></div><form @submit.prevent="saveGroup"><label>分组名称<input v-model="groupForm.name" required maxlength="80" placeholder="例如：华东节点"></label><label>分组说明<textarea v-model="groupForm.description" rows="3" placeholder="部署区域、用途或网络说明"></textarea></label><label class="remember-row"><input v-model="groupForm.enabled" type="checkbox"><span>启用此分组</span></label><label class="remember-row"><input v-model="groupForm.isDefault" type="checkbox"><span>设为默认分组</span></label><button class="primary wide" :disabled="saving">{{ saving ? "保存中…" : "保存分组" }}</button></form></section></div>

  <div v-if="registrationModal" class="modal-backdrop" @click.self="registrationModal = false"><section class="modal-card registration-card"><div class="drawer-head"><div><p class="eyebrow">REGISTER NODE</p><h2>注册节点</h2><p class="muted">节点将加入“{{ selectedGroup?.name }}”</p></div><button class="icon-button" @click="registrationModal = false"><X /></button></div><template v-if="!tokenResult"><form @submit.prevent="createToken"><label>所属分组<select v-model="tokenForm.groupId"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label><label>Token 有效期（分钟）<input v-model.number="tokenForm.expiresMinutes" type="number" min="1" max="1440"></label><button class="primary wide" :disabled="saving">{{ saving ? "生成中…" : "生成注册 Token" }}</button></form></template><div v-else class="alert warning"><b>Token 仅显示一次，请立即完成安装</b><p class="mono">{{ tokenResult.token }}</p><template v-for="(command, platform) in tokenResult.commands" :key="platform"><small>{{ platform }}</small><div class="copy-line"><code>{{ command }}</code><button class="icon-button" title="复制安装命令" @click="copy(String(command))"><Copy /></button></div></template></div></section></div>
</template>
