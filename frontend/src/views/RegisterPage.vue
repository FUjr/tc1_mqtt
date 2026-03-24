<script setup>
import { ref, onMounted, computed } from 'vue'

const registerLevel = ref(1)
const contactInfo = ref('')
const loading = ref(false)
const message = ref('')
const messageType = ref('')

// 注册表单
const form = ref({ username: '', password: '', mac: '', code: '' })
// 邮箱验证状态
const codeSent = ref(false)
const codeLoading = ref(false)
const codeCooldown = ref(0)
let cooldownTimer = null

// 追加MAC表单
const addMacMode = ref(false)
const addMacForm = ref({ username: '', password: '', mac: '' })

// 注册/追加成功后保留数据，用于一键添加配置
const registeredData = ref(null)   // { username, password, mac }
const addedMacData = ref(null)     // { username, password, mac }
const quickAddDone = ref(false)
const quickAddMacDone = ref(false)

onMounted(async () => {
  try {
    const res = await fetch('/api/register/status')
    const data = await res.json()
    registerLevel.value = data.register_level ?? 1
    contactInfo.value = data.contact_info || ''
  } catch (e) {
    console.error('获取注册状态失败', e)
  }
})

const isEmailMode = computed(() => registerLevel.value === 2)

const macRegex = /^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$/

const sendCode = async () => {
  const email = form.value.username.trim()
  if (!email) { message.value = '请先填写邮箱'; messageType.value = 'error'; return }
  const emailRe = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/
  if (!emailRe.test(email)) { message.value = '邮箱格式不正确'; messageType.value = 'error'; return }

  codeLoading.value = true
  message.value = ''
  try {
    const res = await fetch('/api/register/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    })
    const data = await res.json()
    if (res.ok) {
      codeSent.value = true
      message.value = data.message
      messageType.value = 'success'
      codeCooldown.value = 60
      cooldownTimer = setInterval(() => {
        codeCooldown.value--
        if (codeCooldown.value <= 0) {
          clearInterval(cooldownTimer)
          codeCooldown.value = 0
        }
      }, 1000)
    } else {
      message.value = data.error || '发送失败'
      messageType.value = 'error'
    }
  } catch (e) {
    message.value = '网络错误'
    messageType.value = 'error'
  } finally {
    codeLoading.value = false
  }
}

const submit = async () => {
  message.value = ''
  const f = form.value

  if (!f.username || !f.password || !f.mac) {
    message.value = '请填写所有字段'; messageType.value = 'error'; return
  }

  if (isEmailMode.value) {
    const emailRe = /^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/
    if (!emailRe.test(f.username.trim())) {
      message.value = '当前注册限制使用邮箱作为用户名'; messageType.value = 'error'; return
    }
    if (!f.code) {
      message.value = '请填写验证码'; messageType.value = 'error'; return
    }
  } else {
    if (f.username.trim().length < 3) {
      message.value = '用户名至少3个字符'; messageType.value = 'error'; return
    }
  }

  if (!macRegex.test(f.mac.trim())) {
    message.value = 'MAC地址格式不正确，如: D0BAE4618631 或 AA:BB:CC:DD:EE:FF'
    messageType.value = 'error'; return
  }

  loading.value = true
  try {
    const res = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: f.username.trim(), password: f.password, mac: f.mac.trim(), code: f.code })
    })
    const data = await res.json()
    if (res.ok) {
      message.value = data.message || '注册成功！'
      messageType.value = 'success'
      registeredData.value = { username: f.username.trim(), password: f.password, mac: f.mac.trim() }
      quickAddDone.value = false
      form.value = { username: '', password: '', mac: '', code: '' }
      codeSent.value = false
    } else {
      message.value = data.error || '注册失败'
      messageType.value = 'error'
    }
  } catch (e) {
    message.value = '网络错误，请重试'; messageType.value = 'error'
  } finally {
    loading.value = false
  }
}

const submitAddMac = async () => {
  message.value = ''
  const f = addMacForm.value
  if (!f.username || !f.password || !f.mac) {
    message.value = '请填写所有字段'; messageType.value = 'error'; return
  }
  if (!macRegex.test(f.mac.trim())) {
    message.value = 'MAC地址格式不正确'; messageType.value = 'error'; return
  }
  loading.value = true
  try {
    const res = await fetch('/api/register/add-mac', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: f.username.trim(), password: f.password, mac: f.mac.trim() })
    })
    const data = await res.json()
    if (res.ok) {
      message.value = data.message || 'MAC地址已添加！'
      messageType.value = 'success'
      addedMacData.value = { username: f.username.trim(), password: f.password, mac: f.mac.trim() }
      quickAddMacDone.value = false
      addMacForm.value = { username: '', password: '', mac: '' }
    } else {
      message.value = data.error || '添加失败'
      messageType.value = 'error'
    }
  } catch (e) {
    message.value = '网络错误，请重试'; messageType.value = 'error'
  } finally {
    loading.value = false
  }
}
const applyQuickConfig = (data) => {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const host = `${proto}://${window.location.hostname}/mqtt`

  const mqttConfig = {
    host,
    username: data.username,
    password: data.password,
    clientId: 'web_client_' + Math.random().toString(16).substr(2, 8)
  }
  localStorage.setItem('mqtt_config', JSON.stringify(mqttConfig))

  const normalizedMac = data.mac.replace(/[:-]/g, '').toUpperCase()

  let devices = []
  try {
    const saved = localStorage.getItem('ztc1_devices')
    if (saved) devices = JSON.parse(saved)
  } catch (e) {}

  for (const version of ['v1', 'v2']) {
    const exists = devices.find(d =>
      d.mac.replace(/[:-]/g, '').toUpperCase() === normalizedMac && d.version === version
    )
    if (!exists) devices.push({ mac: normalizedMac, version })
  }
  localStorage.setItem('ztc1_devices', JSON.stringify(devices))
}

const doQuickAdd = () => {
  applyQuickConfig(registeredData.value)
  quickAddDone.value = true
}

const doQuickAddMac = () => {
  applyQuickConfig(addedMacData.value)
  quickAddMacDone.value = true
}
</script>

<template>
  <div class="container">
    <div class="header-bar">
      <h1>MQTT 用户注册</h1>
      <div class="header-actions">
        <router-link to="/" class="nav-link">控制台</router-link>
        <router-link to="/admin" class="nav-link">管理</router-link>
      </div>
    </div>

    <!-- 关闭注册 -->
    <div v-if="registerLevel === 0" class="card">
      <p class="closed-msg">注册功能已关闭</p>
      <p v-if="contactInfo" class="contact">{{ contactInfo }}</p>
    </div>

    <!-- 注册表单 -->
    <div v-else class="card">
      <!-- 切换标签 -->
      <div class="tabs">
        <button :class="['tab', { active: !addMacMode }]" @click="addMacMode = false; message = ''">注册账号</button>
        <button :class="['tab', { active: addMacMode }]" @click="addMacMode = true; message = ''">追加MAC地址</button>
      </div>

      <!-- 注册须知 -->
      <div v-if="!addMacMode" class="notice">
        <p class="notice-title">📋 注册须知</p>
        <ul>
          <li>密码一旦创建<strong>无法自行修改</strong>，请妥善保管</li>
          <li>每个 MAC 地址全局唯一，<strong>不可冲突</strong></li>
          <li>用户名全局唯一，<strong>不可冲突</strong></li>
          <li>维护者有权随时<strong>移除账号</strong></li>
          <li v-if="contactInfo">如 MAC 已被添加或需改密码，请联系维护者：<strong>{{ contactInfo }}</strong></li>
        </ul>
      </div>

      <!-- 注册表单 -->
      <template v-if="!addMacMode">
        <div class="form-group">
          <label>{{ isEmailMode ? '邮箱（作为用户名）' : '用户名' }}</label>
          <div v-if="isEmailMode" class="input-row">
            <input v-model="form.username" :placeholder="isEmailMode ? 'user@example.com' : '3-32个字符'" />
            <button class="code-btn" @click="sendCode" :disabled="codeLoading || codeCooldown > 0">
              {{ codeCooldown > 0 ? `${codeCooldown}s` : (codeLoading ? '发送中' : '发送验证码') }}
            </button>
          </div>
          <input v-else v-model="form.username" placeholder="3-32个字符" maxlength="32" />
        </div>
        <div v-if="isEmailMode" class="form-group">
          <label>验证码</label>
          <input v-model="form.code" placeholder="请输入邮件中的6位验证码" maxlength="6" />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input v-model="form.password" type="password" placeholder="至少6个字符" />
        </div>
        <div class="form-group">
          <label>设备 MAC 地址</label>
          <input v-model="form.mac" placeholder="D0BAE4618631 或 AA:BB:CC:DD:EE:FF" maxlength="17" />
        </div>
        <div v-if="message" :class="['msg', messageType]">{{ message }}</div>
        <div v-if="registeredData" class="quick-add-wrap">
          <button class="quick-add-btn" @click="doQuickAdd" :disabled="quickAddDone">
            {{ quickAddDone ? '✔ 配置已保存' : '一键添加配置' }}
          </button>
          <span class="quick-add-hint">自动写入服务器、用户名、v1 v2 设备到本地配置</span>
        </div>
        <button class="submit-btn" @click="submit" :disabled="loading">
          {{ loading ? '注册中...' : '注册' }}
        </button>
      </template>

      <!-- 追加MAC表单 -->
      <template v-else>
        <div class="notice">
          <p class="notice-title">📌 追加 MAC 地址</p>
          <ul>
            <li>输入你的用户名和密码以验证身份</li>
            <li>新 MAC 地址不可与已有记录冲突</li>
            <li v-if="contactInfo">如遇问题请联系：<strong>{{ contactInfo }}</strong></li>
          </ul>
        </div>
        <div class="form-group">
          <label>用户名</label>
          <input v-model="addMacForm.username" placeholder="你的用户名" />
        </div>
        <div class="form-group">
          <label>密码</label>
          <input v-model="addMacForm.password" type="password" placeholder="你的密码" />
        </div>
        <div class="form-group">
          <label>新 MAC 地址</label>
          <input v-model="addMacForm.mac" placeholder="D0BAE4618631 或 AA:BB:CC:DD:EE:FF" maxlength="17" />
        </div>
        <div v-if="message" :class="['msg', messageType]">{{ message }}</div>
        <div v-if="addedMacData" class="quick-add-wrap">
          <button class="quick-add-btn" @click="doQuickAddMac" :disabled="quickAddMacDone">
            {{ quickAddMacDone ? '✔ 配置已保存' : '一键添加配置' }}
          </button>
          <span class="quick-add-hint">自动写入服务器、用户名、v1 v2 设备到本地配置</span>
        </div>
        <button class="submit-btn" @click="submitAddMac" :disabled="loading">
          {{ loading ? '提交中...' : '追加 MAC' }}
        </button>
      </template>
    </div>
  </div>
</template>

<style scoped>
.card {
  max-width: 480px;
  margin: 30px auto;
  background: var(--bg-card);
  border-radius: 8px;
  padding: 28px;
  box-shadow: 0 2px 8px var(--shadow-color);
}
/* Tabs */
.tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 20px;
  border-bottom: 2px solid var(--border-color);
  padding-bottom: 2px;
}
.tab {
  padding: 7px 16px;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.95em;
  color: var(--text-secondary);
  border-radius: 6px 6px 0 0;
}
.tab.active {
  color: #2196f3;
  background: var(--bg-secondary);
  font-weight: 600;
}
/* Notice */
.notice {
  background: var(--bg-secondary);
  border-left: 3px solid #ff9800;
  border-radius: 0 6px 6px 0;
  padding: 10px 14px;
  margin-bottom: 20px;
  font-size: 0.88em;
}
.notice-title {
  font-weight: 600;
  margin: 0 0 6px;
  color: var(--text-primary);
}
.notice ul {
  margin: 0;
  padding-left: 18px;
  color: var(--text-secondary);
  line-height: 1.8;
}
.notice strong { color: var(--text-primary); }
/* Form */
.form-group { margin-bottom: 16px; }
.form-group label {
  display: block;
  margin-bottom: 6px;
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.92em;
}
.form-group input, .input-row input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 1em;
  box-sizing: border-box;
}
.form-group input:focus, .input-row input:focus {
  outline: none;
  border-color: #2196f3;
  box-shadow: 0 0 0 2px rgba(33,150,243,.2);
}
.input-row {
  display: flex;
  gap: 8px;
}
.input-row input { flex: 1; }
.code-btn {
  padding: 0 14px;
  background: #2196f3;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  white-space: nowrap;
  font-size: 0.88em;
  transition: background 0.2s;
}
.code-btn:hover:not(:disabled) { background: #1976d2; }
.code-btn:disabled { opacity: 0.6; cursor: not-allowed; }
.submit-btn {
  width: 100%;
  padding: 11px;
  background: #4caf50;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 1em;
  cursor: pointer;
  transition: background 0.2s;
  margin-top: 4px;
}
.submit-btn:hover:not(:disabled) { background: #43a047; }
.submit-btn:disabled { opacity: 0.6; cursor: not-allowed; }
.msg {
  padding: 10px;
  border-radius: 6px;
  margin-bottom: 12px;
  font-size: 0.9em;
}
.msg.success { background: #e8f5e9; color: #2e7d32; }
.msg.error { background: #fbe9e7; color: #c62828; }
.closed-msg {
  text-align: center;
  color: var(--text-muted);
  font-size: 1.1em;
  margin-bottom: 8px;
}
.contact { text-align: center; color: var(--text-secondary); font-size: 0.9em; }
.quick-add-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.quick-add-btn {
  padding: 9px 18px;
  background: #2196f3;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 0.95em;
  cursor: pointer;
  transition: background 0.2s;
  white-space: nowrap;
}
.quick-add-btn:hover:not(:disabled) { background: #1976d2; }
.quick-add-btn:disabled { background: #66bb6a; cursor: default; }
.quick-add-hint { font-size: 0.82em; color: var(--text-secondary); }
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
.nav-link:hover { background: var(--bg-hover); }
.container { max-width: 1200px; margin: 0 auto; padding: 20px; }
h1 { text-align: center; color: var(--text-heading); margin: 0; }
</style>
