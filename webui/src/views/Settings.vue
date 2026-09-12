<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { api, ApiError } from "../api/client";
import { useSessionStore } from "../stores/session";
import { Plus, Pencil, Trash2, RefreshCw, RotateCw, X, Copy, KeyRound, Users, Server, Shield, Network, UserRoundCheck } from "@lucide/vue-next";

type ModalType = "key" | "user" | "account" | "profile" | "scope" | "grant" | "";
const session = useSessionStore();
const keys=ref<any[]>([]),users=ref<any[]>([]),accounts=ref<any[]>([]),profiles=ref<any[]>([]),scopes=ref<any[]>([]),grants=ref<any[]>([]);
const error=ref(""),notice=ref(""),modal=ref<ModalType>(""),editingId=ref(0),saving=ref(false),revealed=ref<any>();
const registrationOpen=ref(false),delegationDomains=ref<any[]>([]),scopeDomains=ref<any[]>([]),scopeDomainsLoading=ref(false);
const scopeNodePrefix=ref("");
const keyForm=ref({name:"",scopes:["dns:read"]}),userForm=ref({username:"",password:"",role:"user",enabled:true});
const accountForm=ref({name:"",providerType:"Ali",accessKeyId:"",secret:"",enabled:true});
const profileForm=ref({name:"",email:"",caDirUrl:"",isDefault:false,enabled:true});
const scopeForm=ref({dnsAccountId:0,providerDomainId:"",zoneName:"",fqdn:"",inheritChildren:true});
const grantForm=ref({userId:0,scopeId:0,level:"viewer"});
const delegationForm=ref({apply_account:"",apply_domain_id:"",apply_domain_name:"",apply_prefix:""});
const isAdmin=computed(()=>session.user?.role==="admin");
const modalTitle=computed(()=>{if(!modal.value)return "设置";const titles:Record<Exclude<ModalType,"">,string>={key:"创建 AccessKey",user:editingId.value?"编辑用户":"创建用户",account:editingId.value?"编辑 DNS 账号":"创建 DNS 账号",profile:editingId.value?"编辑 ACME Profile":"创建 ACME Profile",scope:editingId.value?"编辑域名节点":"创建域名节点",grant:editingId.value?"编辑域名授权":"创建域名授权"};return titles[modal.value]});
const userName=(id:number)=>users.value.find(v=>v.id===id)?.username||`用户 #${id}`;
const scopeName=(id:number)=>scopes.value.find(v=>v.id===id)?.fqdn||`节点 #${id}`;
const accountName=(id:number)=>accounts.value.find(v=>v.id===id)?.name||`账号 #${id}`;
const permissionLabel=(level:string)=>({viewer:"仅查看",dns_editor:"DNS 编辑",certificate_manager:"证书管理",owner:"节点所有者"}[level]||level);
function fail(e:unknown){error.value=e instanceof ApiError?`${e.message} · ${e.requestId}`:"操作失败"}
function success(message:string){notice.value=message;error.value="";window.setTimeout(()=>notice.value="",1800)}
async function load(){try{
  keys.value=await api<any[]>("/api/access-keys");
  if(isAdmin.value){[users.value,accounts.value,profiles.value,scopes.value,grants.value]=await Promise.all([api<any[]>("/api/admin/users"),api<any[]>("/api/dns-accounts"),api<any[]>("/api/acme-profiles"),api<any[]>("/api/domain-scopes"),api<any[]>("/api/domain-grants")]);
    const registration=await api<any[]>("/api/settings/registration");registrationOpen.value=registration.find(v=>v.key==="registration.open")?.value==="true";
    const settings=await api<any[]>("/api/settings/certificate"),value=(key:string)=>settings.find(v=>v.key===`certificate.${key}`)?.value||"";
    delegationForm.value={apply_account:value("apply_account"),apply_domain_id:value("apply_domain_id"),apply_domain_name:value("apply_domain_name"),apply_prefix:value("apply_prefix")};
    if(delegationForm.value.apply_account)await loadDelegationDomains(false);
  }
}catch(e){fail(e)}}
async function open(type:ModalType,row?:any){modal.value=type;editingId.value=row?.id||0;revealed.value=undefined;
  if(type==="key")keyForm.value={name:"",scopes:["dns:read"]};
  if(type==="user")userForm.value={username:row?.username||"",password:"",role:row?.role||"user",enabled:row?.enabled??true};
  if(type==="account")accountForm.value={name:row?.name||"",providerType:row?.providerType||"Ali",accessKeyId:"",secret:"",enabled:row?.enabled??true};
  if(type==="profile")profileForm.value={name:row?.name||"",email:row?.email||"",caDirUrl:row?.caDirUrl||"",isDefault:row?.isDefault||false,enabled:row?.enabled??true};
  if(type==="scope"){
    scopeForm.value={dnsAccountId:row?.dnsAccountId||0,providerDomainId:row?.providerDomainId||"",zoneName:row?.zoneName||"",fqdn:row?.fqdn||"",inheritChildren:row?.inheritChildren??true};
    await loadScopeDomains(false);
    scopeNodePrefix.value=scopePrefix(scopeForm.value.fqdn,scopeForm.value.zoneName);
  }
  if(type==="grant")grantForm.value={userId:row?.userId||0,scopeId:row?.scopeId||0,level:row?.level||"viewer"};
}
async function saveModal(){if(saving.value)return;saving.value=true;try{
  if(modal.value==="key"){revealed.value=await api("/api/access-keys",{method:"POST",body:JSON.stringify(keyForm.value)});await load();return}
  if(modal.value==="user")await api(editingId.value?`/api/admin/users/${editingId.value}`:"/api/admin/users",{method:editingId.value?"PUT":"POST",body:JSON.stringify(userForm.value)});
  if(modal.value==="account")await api(editingId.value?`/api/dns-accounts/${editingId.value}`:"/api/dns-accounts",{method:editingId.value?"PUT":"POST",body:JSON.stringify(accountForm.value)});
  if(modal.value==="profile")await api(editingId.value?`/api/acme-profiles/${editingId.value}`:"/api/acme-profiles",{method:editingId.value?"PUT":"POST",body:JSON.stringify(profileForm.value)});
  if(modal.value==="scope"){
    updateScopeFQDN();
    await api(editingId.value?`/api/domain-scopes/${editingId.value}`:"/api/domain-scopes",{method:editingId.value?"PUT":"POST",body:JSON.stringify({...scopeForm.value,id:editingId.value})});
  }
  if(modal.value==="grant")await api("/api/domain-grants",{method:"POST",body:JSON.stringify(grantForm.value)});
  modal.value="";await load();success("设置已保存");
}catch(e){fail(e)}finally{saving.value=false}}
async function remove(url:string,message:string){if(!confirm(message))return;try{await api(url,{method:"DELETE"});await load();success("已删除")}catch(e){fail(e)}}
async function rotateKey(id:number){if(!confirm("轮换后旧 Secret 立即失效，继续？"))return;try{revealed.value=await api(`/api/access-keys/${id}/rotate`,{method:"POST"});modal.value="key";await load()}catch(e){fail(e)}}
async function accountAction(id:number,action:string){try{const r:any=await api(`/api/dns-accounts/${id}/${action}`,{method:"POST"});success(action==="test"?"连接测试成功":`已同步 ${r.synced||0} 个域名`);if(action==="sync")await load()}catch(e){fail(e)}}
async function saveRegistration(){try{await api("/api/settings/registration",{method:"PUT",body:JSON.stringify({open:registrationOpen.value})});success("注册策略已保存")}catch(e){fail(e)}}
async function loadDelegationDomains(reset=true){delegationDomains.value=[];if(reset){delegationForm.value.apply_domain_id="";delegationForm.value.apply_domain_name=""}if(!delegationForm.value.apply_account)return;try{const r:any=await api(`/api/${encodeURIComponent(delegationForm.value.apply_account)}/domains?pageNumber=1&pageSize=100`);delegationDomains.value=Array.isArray(r?.domains)?r.domains:[]}catch(e){fail(e)}}
function selectDelegationDomain(){delegationForm.value.apply_domain_name=delegationDomains.value.find(v=>v.id===delegationForm.value.apply_domain_id)?.domainName||""}
async function loadScopeDomains(reset=true){
  scopeDomains.value=[];
  if(reset){scopeForm.value.providerDomainId="";scopeForm.value.zoneName="";scopeForm.value.fqdn="";scopeNodePrefix.value=""}
  const selected=accounts.value.find(v=>Number(v.id)===Number(scopeForm.value.dnsAccountId));
  if(!selected)return;
  scopeDomainsLoading.value=true;
  try{
    const r:any=await api(`/api/${encodeURIComponent(selected.name)}/domains?pageNumber=1&pageSize=100`);
    scopeDomains.value=Array.isArray(r?.domains)?r.domains:[];
  }catch(e){fail(e)}finally{scopeDomainsLoading.value=false}
}
function selectScopeDomain(){
  const selected=scopeDomains.value.find(v=>String(v.id)===String(scopeForm.value.providerDomainId));
  scopeForm.value.zoneName=selected?.domainName||"";
  scopeNodePrefix.value="";
  updateScopeFQDN();
}
function scopePrefix(fqdn:string,zone:string){
  const normalizedFQDN=String(fqdn||"").replace(/\.$/,"").toLowerCase();
  const normalizedZone=String(zone||"").replace(/\.$/,"").toLowerCase();
  if(!normalizedZone||normalizedFQDN===normalizedZone)return "";
  return normalizedFQDN.endsWith(`.${normalizedZone}`)?normalizedFQDN.slice(0,-normalizedZone.length-1):"";
}
function updateScopeFQDN(){
  const prefix=scopeNodePrefix.value.trim().replace(/^\.+|\.+$/g,"").toLowerCase();
  scopeNodePrefix.value=prefix;
  scopeForm.value.fqdn=prefix?`${prefix}.${scopeForm.value.zoneName}`:scopeForm.value.zoneName;
}
async function saveDelegation(){try{await api("/api/settings/certificate",{method:"PUT",body:JSON.stringify(delegationForm.value)});success("委托配置已保存")}catch(e){fail(e)}}
async function copySecret(){if(revealed.value?.secret)await navigator.clipboard.writeText(revealed.value.secret)}
onMounted(load);
</script>

<template>
  <div v-if="error" class="alert error">{{error}}</div><div v-if="notice" class="alert success">{{notice}}</div>
  <section class="card settings-list"><div class="settings-head"><div><h3><KeyRound/>API AccessKey</h3><p class="muted">用于自动化调用，可随时轮换或撤销。</p></div><button class="primary" @click="open('key')"><Plus/>创建</button></div><div v-if="!keys.length" class="empty compact">暂无 AccessKey</div><div v-for="k in keys" :key="k.id" class="setting-row"><span><b>{{k.name}}</b><small class="mono">{{k.keyId}} · Secret 尾号 {{k.secretTail}}</small><small>{{k.scopes}} · {{k.enabled?'启用':'已撤销'}}</small></span><span class="row-actions"><button class="icon-button" title="轮换" @click="rotateKey(k.id)"><RotateCw/></button><button class="icon-button danger" title="撤销" @click="remove(`/api/access-keys/${k.id}`,'撤销后无法恢复，继续？')"><Trash2/></button></span></div></section>
  <template v-if="isAdmin">
    <section class="card settings-list"><div class="settings-head"><div><h3><Users/>用户管理</h3><p class="muted">管理角色、状态及初始密码。</p></div><button class="primary" @click="open('user')"><Plus/>创建</button></div><div v-for="u in users" :key="u.id" class="setting-row"><span><b>{{u.username}}</b><small>{{u.role==='admin'?'管理员':'普通用户'}} · {{u.enabled?'启用':'停用'}} · {{u.mustChangePassword?'待修改密码':'密码正常'}}</small></span><button class="icon-button" title="编辑" @click="open('user',u)"><Pencil/></button></div></section>
    <section class="card settings-list"><div class="settings-head"><div><h3><Server/>DNS 厂商账号</h3><p class="muted">凭据仅显示配置状态与尾号。</p></div><button class="primary" @click="open('account')"><Plus/>创建</button></div><div v-if="!accounts.length" class="empty compact">暂无 DNS 账号</div><div v-for="a in accounts" :key="a.id" class="setting-row"><span><b>{{a.name}}</b><small>{{a.providerType}} · 凭据尾号 {{a.credentialTail||'—'}} · v{{a.version}} · {{a.enabled?'启用':'停用'}}</small></span><span class="row-actions"><button class="secondary" @click="accountAction(a.id,'test')">测试</button><button class="secondary" @click="accountAction(a.id,'sync')"><RefreshCw/>同步</button><button class="icon-button" title="编辑" @click="open('account',a)"><Pencil/></button><button class="icon-button danger" title="删除" @click="remove(`/api/dns-accounts/${a.id}`,'账号仍被引用时不会删除。继续？')"><Trash2/></button></span></div></section>
    <section class="card settings-list"><div class="settings-head"><div><h3><Shield/>ACME Profile</h3><p class="muted">证书签发邮箱与 CA 账户。</p></div><button class="primary" @click="open('profile')"><Plus/>创建</button></div><div v-if="!profiles.length" class="empty compact">暂无 ACME Profile</div><div v-for="p in profiles" :key="p.id" class="setting-row"><span><b>{{p.name}}</b><small>{{p.email}} · {{p.isDefault?'默认':'备用'}} · 已签发 {{p.issuedCount||0}} · {{p.enabled?'启用':'停用'}}</small></span><span class="row-actions"><button class="icon-button" title="编辑" @click="open('profile',p)"><Pencil/></button><button class="icon-button danger" title="删除" @click="remove(`/api/acme-profiles/${p.id}`,'存在运行任务时不会删除。继续？')"><Trash2/></button></span></div></section>
    <section class="card settings-list"><div class="settings-head"><div><h3><Network/>域名权限节点</h3><p class="muted">映射真实 Zone 的二级、三级授权范围。</p></div><button class="primary" @click="open('scope')"><Plus/>创建</button></div><div v-if="!scopes.length" class="empty compact">暂无权限节点</div><div v-for="s in scopes" :key="s.id" class="setting-row"><span><b class="mono">{{s.fqdn}}</b><small>{{accountName(s.dnsAccountId)}} · Zone {{s.zoneName}} · {{s.inheritChildren?'包含子域':'仅当前节点'}}</small></span><span class="row-actions"><button class="icon-button" title="编辑" @click="open('scope',s)"><Pencil/></button><button class="icon-button danger" title="删除" @click="remove(`/api/domain-scopes/${s.id}`,'仍有用户授权时不会删除。继续？')"><Trash2/></button></span></div></section>
    <section class="card settings-list"><div class="settings-head"><div><h3><UserRoundCheck/>域名授权</h3><p class="muted">用户对权限节点的访问级别。</p></div><button class="primary" @click="open('grant')"><Plus/>授权</button></div><div v-if="!grants.length" class="empty compact">暂无域名授权</div><div v-for="g in grants" :key="g.id" class="setting-row"><span><b>{{userName(g.userId)}} → <span class="mono">{{scopeName(g.scopeId)}}</span></b><small>{{permissionLabel(g.level)}}</small></span><span class="row-actions"><button class="icon-button" title="编辑" @click="open('grant',g)"><Pencil/></button><button class="icon-button danger" title="撤销" @click="remove(`/api/domain-grants/${g.id}`,'确定撤销此域名授权？')"><Trash2/></button></span></div></section>
    <section class="card"><div class="settings-head"><div><h3>注册策略</h3><p class="muted">默认关闭公开注册。</p></div><button class="primary" @click="saveRegistration">保存</button></div><label class="remember-row"><input v-model="registrationOpen" type="checkbox"><span>允许公开注册普通用户</span></label></section>
    <section class="card"><div class="settings-head"><div><h3>CNAME 委托承载域名</h3><p class="muted">外部证书的 DNS-01 TXT 承载位置。</p></div><button class="primary" @click="saveDelegation">保存</button></div><div class="form-grid"><select v-model="delegationForm.apply_account" @change="loadDelegationDomains(true)"><option value="">选择 DNS 账号</option><option v-for="a in accounts" :value="a.name">{{a.name}} · {{a.providerType}}</option></select><select v-model="delegationForm.apply_domain_id" @change="selectDelegationDomain"><option value="">选择承载域名</option><option v-for="d in delegationDomains" :value="d.id">{{d.domainName}}</option></select><input v-model="delegationForm.apply_prefix" placeholder="可选前缀，如 acme"></div><p v-if="delegationForm.apply_domain_name" class="mono muted">{{delegationForm.apply_prefix?delegationForm.apply_prefix+'.':''}}&lt;哈希&gt;.{{delegationForm.apply_domain_name}}.</p></section>
  </template>

  <div v-if="modal" class="modal-backdrop" @click.self="modal=''" ><section class="modal-card"><div class="drawer-head"><div><p class="eyebrow">SETTINGS</p><h2>{{modalTitle}}</h2></div><button class="icon-button" @click="modal='' "><X/></button></div>
    <div v-if="revealed" class="alert warning"><b>Secret 仅显示一次</b><p class="mono">{{revealed.accessKey?.keyId}}</p><p class="mono">{{revealed.secret}}</p><button class="secondary" @click="copySecret"><Copy/>复制 Secret</button></div>
    <form v-else @submit.prevent="saveModal">
      <template v-if="modal==='key'"><label>名称<input v-model="keyForm.name" required></label><fieldset class="scope-fieldset"><legend>权限范围</legend><div class="scope-checks"><label><input v-model="keyForm.scopes" type="checkbox" value="dns:read"><span><b>DNS 读取</b><small>查看获授权域名及解析记录</small></span></label><label><input v-model="keyForm.scopes" type="checkbox" value="dns:write"><span><b>DNS 修改</b><small>创建、编辑和删除解析记录</small></span></label><label><input v-model="keyForm.scopes" type="checkbox" value="certificate:issue"><span><b>证书申请</b><small>为获授权域名发起证书申请</small></span></label><label><input v-model="keyForm.scopes" type="checkbox" value="certificate:renew"><span><b>证书续期</b><small>为现有证书创建续期任务</small></span></label><label><input v-model="keyForm.scopes" type="checkbox" value="certificate:download"><span><b>证书下载</b><small>下载证书、私钥和完整包</small></span></label></div></fieldset></template>
      <template v-if="modal==='user'"><label>用户名<input v-model="userForm.username" :disabled="!!editingId" required></label><label>{{editingId?'重置密码（留空不修改）':'初始密码'}}<input v-model="userForm.password" type="password" :required="!editingId" minlength="10"></label><label>角色<select v-model="userForm.role"><option value="user">普通用户</option><option value="admin">管理员</option></select></label><label class="remember-row"><input v-model="userForm.enabled" type="checkbox"><span>启用用户</span></label></template>
      <template v-if="modal==='account'"><label>账号名称<input v-model="accountForm.name" required></label><label>厂商<select v-model="accountForm.providerType"><option>Ali</option><option>Tencent</option><option>Cloudflare</option></select></label><label>AccessKey ID<input v-model="accountForm.accessKeyId" :required="!editingId" :placeholder="editingId?'留空保持原凭据':''"></label><label>Secret<input v-model="accountForm.secret" type="password" :required="!editingId" :placeholder="editingId?'留空保持原凭据':''"></label><label class="remember-row"><input v-model="accountForm.enabled" type="checkbox"><span>启用账号</span></label></template>
      <template v-if="modal==='profile'"><label>名称<input v-model="profileForm.name" required></label><label>邮箱<input v-model="profileForm.email" type="email" required></label><label>CA Directory URL<input v-model="profileForm.caDirUrl"></label><label class="remember-row"><input v-model="profileForm.isDefault" type="checkbox"><span>默认 Profile</span></label><label class="remember-row"><input v-model="profileForm.enabled" type="checkbox"><span>启用</span></label></template>
      <template v-if="modal==='scope'"><label>DNS 账号<select v-model="scopeForm.dnsAccountId" required @change="loadScopeDomains(true)"><option :value="0">选择账号</option><option v-for="a in accounts" :key="a.id" :value="a.id">{{a.name}} · {{a.providerType}}</option></select></label><label>真实 Zone<select v-model="scopeForm.providerDomainId" required :disabled="!scopeForm.dnsAccountId||scopeDomainsLoading" @change="selectScopeDomain"><option value="">{{scopeDomainsLoading?'正在加载…':scopeForm.dnsAccountId?'选择已同步域名':'请先选择 DNS 账号'}}</option><option v-for="d in scopeDomains" :key="d.id" :value="String(d.id)">{{d.domainName}} · {{d.dnsFrom||accountName(scopeForm.dnsAccountId)}}</option></select></label><div v-if="scopeForm.zoneName" class="selection-summary"><small>Provider Domain ID</small><b class="mono">{{scopeForm.providerDomainId}}</b><small>真实 Zone</small><b class="mono">{{scopeForm.zoneName}}</b></div><label>授权节点<div class="domain-composer"><input v-model="scopeNodePrefix" :disabled="!scopeForm.zoneName" placeholder="dev 或 api.dev" pattern="[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*"><span v-if="scopeForm.zoneName" class="mono">.{{scopeForm.zoneName}}</span></div></label><p class="muted compact-note">只需填写节点前缀；留空表示授权整个 Zone，固定后缀不可修改。</p><label class="remember-row"><input v-model="scopeForm.inheritChildren" type="checkbox"><span>包含全部子域</span></label></template>
      <template v-if="modal==='grant'"><label>用户<select v-model="grantForm.userId" required><option :value="0">选择用户</option><option v-for="u in users" :value="u.id">{{u.username}}</option></select></label><label>域名节点<select v-model="grantForm.scopeId" required><option :value="0">选择节点</option><option v-for="s in scopes" :value="s.id">{{s.fqdn}}</option></select></label><label>权限<select v-model="grantForm.level"><option value="viewer">仅查看</option><option value="dns_editor">DNS 编辑</option><option value="certificate_manager">证书管理</option><option value="owner">节点所有者</option></select></label></template>
      <button class="primary wide" :disabled="saving">{{saving?'保存中…':'保存'}}</button>
    </form>
  </section></div>
</template>
