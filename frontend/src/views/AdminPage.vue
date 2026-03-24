<script setup>
import { ref, onMounted } from 'vue'

// Auth state
const token = ref(localStorage.getItem('mqtt_admin_token') || '')
const loginForm = ref({ username: '', password: '' })
const loginError = ref('')
const loginLoading = ref(false)

// Users
const users = ref([])
const usersLoading = ref(false)

// ACL editor
const aclContent = ref('')
const aclLoading = ref(false)
const aclSaved = ref(false)

// Config
const configData = ref(null)

// New user form
const newUser = ref({ username: '', password: '', mac: '' })
const newUserMsg = ref('')
const newUserMsgType = ref('')

// Edit password modal
const editModal = ref(false)
const editTarget = ref('')
const editPassword = ref('')
const editMsg = ref('')

// Add MAC modal
const addMacModal = ref(false)
const addMacTarget = ref('')
const addMacValue = ref('')
const addMacMsg = ref('')
const addMacLoading = ref(false)

// Config edit
const configEditLevel = ref(null)
const configEditContact = ref('')
const configSaveMsg = ref('')
const configSaveMsgType = ref('')

// Active tab
const activeTab = ref('users')

const authHeader = () => ({ Authorization: `Bearer ${token.value}` })

const apiFetch = async (url, opts = {}) => {
  const res = await fetch(url, {
    ...opts,
    headers: { 'Content-Type': 'application/json', ...authHeader(), ...(opts.headers || {}) }
  })
  const data = await res.json()
  if (res.status === 401) {
    token.value = ''
    localStorage.removeItem('mqtt_admin_token')
  }
  return { ok: res.ok, status: res.status, data }
}

const login = async () => {
  loginError.value = ''
  loginLoading.value = true
  try {
    const res = await fetch('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(loginForm.value)
    })
    const data = await res.json()
    if (res.ok) {
      token.value = data.token
      localStorage.setItem('mqtt_admin_token', data.token)
      await loadAll()
    } else {
      loginError.value = data.error || '登录失败'
    }
  } catch (e) {
    loginError.value = '网络错误'
  } finally {
    loginLoading.value = false
  }
}

const logout = () => {
  token.value = ''
  localStorage.removeItem('mqtt_admin_token')
}

const loadAll = async () => {
  await Promise.all([loadUsers(), loadACL(), loadConfig()])
}

const loadUsers = async () => {
  usersLoading.value = true
  const { ok, data } = await apiFetch('/api/admin/users')
  if (ok) users.value = data || []
  usersLoading.value = false
}

const loadACL = async () => {
  const { ok, data } = await apiFetch('/api/admin/acl')
  if (ok) aclContent.value = data.content || ''
}

const loadConfig = async () => {
  const { ok, data } = await apiFetch('/api/admin/config')
  if (ok) {
    configData.value = data
    configEditLevel.value = data.register_level ?? 1
    configEditContact.value = data.contact_info || ''
  }
}

const saveACL = async () => {
  aclLoading.value = true
  const { ok, data } = await apiFetch('/api/admin/acl', {
    method: 'PUT',
    body: JSON.stringify({ content: aclContent.value })
  })
  if (ok) {
    aclSaved.value = true
    setTimeout(() => aclSaved.value = false, 2000)
  } else {
    alert(data.error || 'ACL保存失败')
  }
  aclLoading.value = false
}

const deleteUser = async (username) => {
  if (!confirm(`确认删除用户 "${username}" 及其ACL规则？`)) return
  const { ok, data } = await apiFetch(`/api/admin/user/${encodeURIComponent(username)}`, { method: 'DELETE' })
  if (ok) {
    await loadAll()
  } else {
    alert(data.error || '删除失败')
  }
}

const openEditModal = (username) => {
  editTarget.value = username
  editPassword.value = ''
  editMsg.value = ''
  editModal.value = true
}

const submitEditPassword = async () => {
  if (!editPassword.value) {
    editMsg.value = '密码不能为空'
    return
  }
  const { ok, data } = await apiFetch(`/api/admin/user/${encodeURIComponent(editTarget.value)}`, {
    method: 'PUT',
    body: JSON.stringify({ password: editPassword.value })
  })
  if (ok) {
    editModal.value = false
    editMsg.value = ''
  } else {
    editMsg.value = data.error || '修改失败'
  }
}

const createUser = async () => {
  newUserMsg.value = ''
  const u = newUser.value
  if (!u.username || !u.password || !u.mac) {
    newUserMsg.value = '所有字段不能为空'
    newUserMsgType.value = 'error'
    return
  }
  const macRegex = /^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$/
  if (!macRegex.test(u.mac)) {
    newUserMsg.value = 'MAC地址格式不正确'
    newUserMsgType.value = 'error'
    return
  }
  const { ok, data } = await apiFetch('/api/admin/users', {
    method: 'POST',
    body: JSON.stringify(u)
  })
  if (ok) {
    newUserMsg.value = '创建成功'
    newUserMsgType.value = 'success'
    newUser.value = { username: '', password: '', mac: '' }
    await loadAll()
  } else {
    newUserMsg.value = data.error || '创建失败'
    newUserMsgType.value = 'error'
  }
}

const saveConfig = async () => {
  configSaveMsg.value = ''
  const level = parseInt(configEditLevel.value)
  if (![0, 1, 2].includes(level)) {
    configSaveMsg.value = '注册等级必须为 0/1/2'
    configSaveMsgType.value = 'error'
    return
  }
  const { ok, data } = await apiFetch('/api/admin/config', {
    method: 'PUT',
    body: JSON.stringify({ register_level: level, contact_info: configEditContact.value })
  })
  if (ok) {
    configData.value.register_level = level
    configData.value.contact_info = configEditContact.value
    configSaveMsg.value = '已保存'
    configSaveMsgType.value = 'success'
    setTimeout(() => configSaveMsg.value = '', 2000)
  } else {
    configSaveMsg.value = data.error || '配置修改失败'
    configSaveMsgType.value = 'error'
  }
}

const openAddMacModal = (username) => {
  addMacTarget.value = username
  addMacValue.value = ''
  addMacMsg.value = ''
  addMacModal.value = true
}

const submitAddMac = async () => {
  addMacMsg.value = ''
  const mac = addMacValue.value.trim()
  const macRegex = /^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$/
  if (!mac || !macRegex.test(mac)) {
    addMacMsg.value = 'MAC地址格式不正确'
    return
  }
  addMacLoading.value = true
  const { ok, data } = await apiFetch(`/api/admin/user/${encodeURIComponent(addMacTarget.value)}`, {
    method: 'PUT',
    body: JSON.stringify({ mac })
  })
  if (ok) {
    addMacModal.value = false
    await loadAll()
  } else {
    addMacMsg.value = data.error || '添加失败'
  }
  addMacLoading.value = false
}

onMounted(() => {
  if (token.value) loadAll()
})
</script>

<template>
  <div class="container">
    <div class="header-bar">
      <h1>MQTT 用户管理</h1>
      <div class="header-actions">
        <router-link to="/" class="nav-link">控制台</router-link>
        <router-link to="/register" class="nav-link">注册</router-link>
        <button v-if="token" class="logout-btn" @click="logout">退出</button>
      </div>
    </div>

    <!-- Login -->
    <div v-if="!token" class="login-card">
      <h2>管理员登录</h2>
      <div class="form-group">
        <label>用户名</label>
        <input v-model="loginForm.username" placeholder="管理员用户名" @keyup.enter="login" />
      </div>
      <div class="form-group">
        <label>密码</label>
        <input v-model="loginForm.password" type="password" placeholder="管理员密码" @keyup.enter="login" />
      </div>
      <div v-if="loginError" class="msg error">{{ loginError }}</div>
      <button class="submit-btn" @click="login" :disabled="loginLoading">
        {{ loginLoading ? '登录中...' : '登录' }}
      </button>
    </div>

    <!-- Admin Panel -->
    <div v-else class="admin-panel">
      <!-- Tabs -->
      <div class="tabs">
        <button :class="['tab', { active: activeTab === 'users' }]" @click="activeTab = 'users'">用户列表</button>
        <button :class="['tab', { active: activeTab === 'create' }]" @click="activeTab = 'create'">创建用户</button>
        <button :class="['tab', { active: activeTab === 'acl' }]" @click="activeTab = 'acl'">ACL编辑</button>
        <button :class="['tab', { active: activeTab === 'config' }]" @click="activeTab = 'config'">配置</button>
      </div>

      <!-- Users Tab -->
      <div v-if="activeTab === 'users'" class="tab-content">
        <div class="tab-header">
          <h3>用户列表</h3>
          <button class="refresh-btn" @click="loadUsers">刷新</button>
        </div>
        <div v-if="usersLoading" class="loading">加载中...</div>
        <div v-else-if="!users || users.length === 0" class="empty">暂无用户</div>
        <table v-else class="user-table">
          <thead>
            <tr>
              <th>用户名</th>
              <th>ACL规则数</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in users" :key="u.username">
              <td class="username-cell">{{ u.username }}</td>
              <td>{{ u.acl_rules ? u.acl_rules.length : 0 }}</td>
              <td class="action-cell">
                <button class="edit-btn" @click="openEditModal(u.username)">改密码</button>
                <button class="mac-btn" @click="openAddMacModal(u.username)">追加MAC</button>
                <button class="del-btn" @click="deleteUser(u.username)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Create User Tab -->
      <div v-if="activeTab === 'create'" class="tab-content">
        <h3>创建新用户</h3>
        <div class="form-group">
          <label>用户名</label>
          <input v-model="newUser.username" placeholder="用户名" />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input v-model="newUser.password" type="password" placeholder="密码" />
        </div>
        <div class="form-group">
          <label>设备MAC地址</label>
          <input v-model="newUser.mac" placeholder="D0BAE4618631 或 AA:BB:CC:DD:EE:FF" />
        </div>
        <div v-if="newUserMsg" :class="['msg', newUserMsgType]">{{ newUserMsg }}</div>
        <button class="submit-btn" @click="createUser">创建</button>
      </div>

      <!-- ACL Tab -->
      <div v-if="activeTab === 'acl'" class="tab-content">
        <div class="tab-header">
          <h3>ACL 文件编辑</h3>
          <button class="refresh-btn" @click="loadACL">刷新</button>
        </div>
        <p class="hint">每个用户的规则以 <code>user &lt;用户名&gt;</code> 开头，后跟 <code>topic readwrite &lt;主题&gt;</code></p>
        <textarea
          v-model="aclContent"
          class="acl-editor"
          spellcheck="false"
        ></textarea>
        <div class="acl-actions">
          <button class="submit-btn" @click="saveACL" :disabled="aclLoading">
            {{ aclLoading ? '保存中...' : (aclSaved ? '✓ 已保存' : '保存并重载') }}
          </button>
        </div>
      </div>

      <!-- Config Tab -->
      <div v-if="activeTab === 'config'" class="tab-content">
        <h3>运行时配置</h3>
        <div v-if="configData">
          <div class="config-item">
            <span class="config-label">容器名称</span>
            <span class="config-value">{{ configData.container_name }}</span>
          </div>
          <div class="config-item">
            <span class="config-label">容器内passwd路径</span>
            <span class="config-value">{{ configData.container_passwd_path }}</span>
          </div>
          <div class="config-item">
            <span class="config-label">主机passwd路径</span>
            <span class="config-value">{{ configData.host_passwd_path }}</span>
          </div>
          <div class="config-item">
            <span class="config-label">ACL文件路径</span>
            <span class="config-value">{{ configData.acl_path }}</span>
          </div>
          <div class="config-item">
            <span class="config-label">ACL模式</span>
            <span class="config-value">
              <span v-for="p in configData.acl_patterns" :key="p" class="pattern-tag">{{ p }}</span>
            </span>
          </div>
          <div class="config-item">
            <span class="config-label">注册等级</span>
            <span class="config-value">
              <select v-model="configEditLevel" class="config-select">
                <option :value="0">0 — 禁止注册</option>
                <option :value="1">1 — 任意注册</option>
                <option :value="2">2 — 邮箱验证</option>
              </select>
            </span>
          </div>
          <div class="config-item">
            <span class="config-label">联系方式</span>
            <span class="config-value" style="font-family:inherit">
              <input v-model="configEditContact" class="config-input" placeholder="如需帮助请联系管理员" />
            </span>
          </div>
          <div style="margin-top:14px">
            <div v-if="configSaveMsg" :class="['msg', configSaveMsgType]" style="margin-bottom:8px">{{ configSaveMsg }}</div>
            <button class="submit-btn" style="width:auto;padding:8px 24px" @click="saveConfig">保存配置</button>
          </div>
        </div>
        <div v-else class="loading">加载中...</div>
      </div>
    </div>

    <!-- Add MAC Modal -->
    <div v-if="addMacModal" class="modal-overlay" @click.self="addMacModal = false">
      <div class="modal">
        <h3>追加 MAC 地址 — {{ addMacTarget }}</h3>
        <div class="form-group">
          <label>MAC 地址</label>
          <input v-model="addMacValue" placeholder="D0BAE4618631 或 AA:BB:CC:DD:EE:FF" @keyup.enter="submitAddMac" />
        </div>
        <div v-if="addMacMsg" class="msg error">{{ addMacMsg }}</div>
        <div class="modal-actions">
          <button class="submit-btn" @click="submitAddMac" :disabled="addMacLoading">{{ addMacLoading ? '提交中...' : '确认追加' }}</button>
          <button class="cancel-btn" @click="addMacModal = false">取消</button>
        </div>
      </div>
    </div>

    <!-- Edit Password Modal -->
    <div v-if="editModal" class="modal-overlay" @click.self="editModal = false">
      <div class="modal">
        <h3>修改密码 - {{ editTarget }}</h3>
        <div class="form-group">
          <label>新密码</label>
          <input v-model="editPassword" type="password" placeholder="输入新密码" @keyup.enter="submitEditPassword" />
        </div>
        <div v-if="editMsg" class="msg error">{{ editMsg }}</div>
        <div class="modal-actions">
          <button class="submit-btn" @click="submitEditPassword">确认修改</button>
          <button class="cancel-btn" @click="editModal = false">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.container {
  max-width: 1100px;
  margin: 0 auto;
  padding: 20px;
}
h1 {
  text-align: center;
  color: var(--text-heading);
  margin: 0;
}
.header-bar {
  position: relative;
  display: flex;
  justify-content: center;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 10px;
}
.header-actions {
  display: flex;
  gap: 4px;
  align-items: center;
}

/* Mobile: header wraps to separate line */
@media (max-width: 600px) {
  .header-bar {
    flex-direction: column;
    align-items: stretch;
  }
  .header-bar h1 {
    margin-bottom: 10px;
  }
  .header-actions {
    justify-content: center;
    flex-wrap: wrap;
  }
}
.nav-link {
  font-size: 0.9em;
  color: var(--text-primary);
  text-decoration: none;
  padding: 4px 10px;
  border-radius: 4px;
  background: var(--bg-secondary);
  transition: background 0.2s;
}
.nav-link:hover {
  background: var(--bg-hover);
}
.logout-btn {
  font-size: 0.9em;
  padding: 4px 10px;
  border-radius: 4px;
  background: #f44336;
  color: white;
  border: none;
  cursor: pointer;
}
.login-card, .admin-panel {
  max-width: 500px;
  margin: 40px auto;
  background: var(--bg-card);
  border-radius: 8px;
  padding: 30px;
  box-shadow: 0 2px 8px var(--shadow-color);
}
.admin-panel {
  max-width: 900px;
  padding: 20px;
}
h2, h3 {
  color: var(--text-heading);
  margin: 0 0 16px;
}
.form-group {
  margin-bottom: 16px;
}
.form-group label {
  display: block;
  margin-bottom: 6px;
  font-weight: 500;
  color: var(--text-secondary);
}
.form-group input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 1em;
  box-sizing: border-box;
}
.form-group input:focus {
  outline: none;
  border-color: #2196f3;
  box-shadow: 0 0 0 2px rgba(33, 150, 243, 0.2);
}
.submit-btn {
  width: 100%;
  padding: 11px;
  background: #2196f3;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 1em;
  cursor: pointer;
  transition: background 0.2s;
}
.submit-btn:hover:not(:disabled) {
  background: #1976d2;
}
.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.msg {
  padding: 10px;
  border-radius: 6px;
  margin-bottom: 12px;
  font-size: 0.9em;
}
.msg.success { background: #e8f5e9; color: #2e7d32; }
.msg.error { background: #fbe9e7; color: #c62828; }
/* Tabs */
.tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 20px;
  border-bottom: 2px solid var(--border-color);
  padding-bottom: 2px;
}
.tab {
  padding: 8px 18px;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.95em;
  color: var(--text-secondary);
  border-radius: 6px 6px 0 0;
  transition: all 0.2s;
}
.tab.active {
  color: #2196f3;
  background: var(--bg-secondary);
  font-weight: 600;
}
.tab-content {
  min-height: 300px;
}
.tab-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.tab-header h3 {
  margin: 0;
}
.refresh-btn {
  padding: 5px 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 5px;
  cursor: pointer;
  color: var(--text-primary);
}
.refresh-btn:hover { background: var(--bg-hover); }
/* User table */
.user-table {
  width: 100%;
  border-collapse: collapse;
}
.user-table th, .user-table td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-light);
}
.user-table th {
  font-weight: 600;
  color: var(--text-secondary);
  background: var(--bg-secondary);
}
.username-cell {
  font-family: monospace;
  font-size: 0.95em;
}
.action-cell {
  display: flex;
  gap: 8px;
}
.edit-btn {
  padding: 4px 10px;
  background: #ff9800;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85em;
}
.edit-btn:hover { background: #f57c00; }
.mac-btn {
  padding: 4px 10px;
  background: #2196f3;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85em;
}
.mac-btn:hover { background: #1976d2; }
.del-btn {
  padding: 4px 10px;
  background: #f44336;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85em;
}
.del-btn:hover { background: #d32f2f; }
/* ACL editor */
.hint {
  font-size: 0.85em;
  color: var(--text-muted);
  margin-bottom: 8px;
}
.hint code {
  background: var(--bg-secondary);
  padding: 1px 5px;
  border-radius: 3px;
}
.acl-editor {
  width: 100%;
  height: 400px;
  font-family: 'Courier New', monospace;
  font-size: 0.9em;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  resize: vertical;
  box-sizing: border-box;
  line-height: 1.5;
}
.acl-actions {
  margin-top: 12px;
}
/* Config */
.config-item {
  display: flex;
  align-items: flex-start;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-light);
  gap: 16px;
}
.config-label {
  min-width: 160px;
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.9em;
}
.config-value {
  color: var(--text-primary);
  font-family: monospace;
  font-size: 0.9em;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.pattern-tag {
  background: var(--bg-secondary);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.9em;
}
/* Toggle switch */
.toggle {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle-slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background: #ccc;
  border-radius: 24px;
  transition: .3s;
}
.toggle-slider::before {
  position: absolute;
  content: "";
  height: 18px; width: 18px;
  left: 3px; bottom: 3px;
  background: white;
  border-radius: 50%;
  transition: .3s;
}
.toggle input:checked + .toggle-slider { background: #4caf50; }
.toggle input:checked + .toggle-slider::before { transform: translateX(20px); }
.config-select {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 0.9em;
  cursor: pointer;
}
.config-input {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 5px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 0.9em;
  width: 280px;
}
/* Modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal {
  background: var(--bg-card);
  border-radius: 8px;
  padding: 28px;
  min-width: 360px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.3);
}
.modal-actions {
  display: flex;
  gap: 10px;
}
.modal-actions .submit-btn {
  flex: 1;
}
.cancel-btn {
  flex: 1;
  padding: 11px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 1em;
  cursor: pointer;
}
.loading, .empty {
  text-align: center;
  color: var(--text-muted);
  padding: 40px;
}
</style>
