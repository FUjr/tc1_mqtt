<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import mqtt from 'mqtt'
import MqttConfig from './components/MqttConfig.vue'
import DeviceControl from './components/DeviceControl.vue'
import ImportExport from './components/ImportExport.vue'

// State
const client = ref(null)
const connected = ref(false)
const devices = reactive({}) // { mac: { ...deviceState, ...localMeta } }
const selectedMac = ref(null)
const newMacInput = ref('')
const deviceList = ref([]) // List of { mac, version } to track
const newDeviceVersion = ref('v1')

// Local Storage Keys
const STORAGE_DEVICES = 'ztc1_devices'

onMounted(() => {
  // Load saved devices
  const saved = localStorage.getItem(STORAGE_DEVICES)
  if (saved) {
    try {
      const parsed = JSON.parse(saved)
      // Migration: old format was plain MAC string array
      if (Array.isArray(parsed) && parsed.length > 0 && typeof parsed[0] === 'string') {
        deviceList.value = parsed.map(mac => ({ mac, version: 'v1' }))
      } else {
        deviceList.value = parsed
      }
    } catch (e) {
      console.error('Failed to load devices', e)
    }
  }
})

// Persist device list
watch(deviceList, (val) => {
  localStorage.setItem(STORAGE_DEVICES, JSON.stringify(val))
}, { deep: true })

const connectMqtt = (config) => {
  if (client.value) {
    client.value.end()
  }

  const { host, username, password, clientId } = config
  console.log(`Connecting to ${host}...`)

  try {
    const mqttClient = mqtt.connect(host, {
      username,
      password,
      clientId,
      clean: true,
      connectTimeout: 4000,
    })

    mqttClient.on('connect', () => {
      console.log('MQTT Connected')
      connected.value = true
      // Subscribe to all tracked devices
      deviceList.value.forEach(entry => subscribeDevice(entry))
    })

    mqttClient.on('message', (topic, message) => {
      const msgStr = message.toString()
      // v2 HA-style topics
      if (topic.startsWith('homeassistant/')) {
        handleV2Message(topic, msgStr)
        return
      }
      // v1 JSON messages
      try {
        const payload = JSON.parse(msgStr)
        handleMessage(topic, payload)
      } catch (e) {
        console.warn('Failed to parse message', e)
      }
    })

    mqttClient.on('error', (err) => {
      console.error('MQTT Error', err)
      connected.value = false
    })

    mqttClient.on('close', () => {
      console.log('MQTT Closed')
      connected.value = false
    })

    client.value = mqttClient
  } catch (e) {
    console.error('Connection factory error', e)
  }

}

const disconnectMqtt = () => {
  if (client.value) {
    client.value.end()
    client.value = null
  }
  connected.value = false
}

const getDeviceVersion = (mac) => {
  const entry = deviceList.value.find(d => d.mac === mac)
  return entry?.version || 'v1'
}

const subscribeDevice = (entry) => {
  if (!client.value || !connected.value) return
  const { mac, version } = typeof entry === 'object' ? entry : { mac: entry, version: getDeviceVersion(entry) }
  
  if (version === 'v2') {
    const topics = [
      `homeassistant/switch/${mac}/+/state`,
      `homeassistant/switch/${mac}/+/config`,
      `homeassistant/sensor/${mac}/+/state`,
      `homeassistant/sensor/${mac}/+/config`,
      `homeassistant/button/${mac}/+/config`,
    ]
    client.value.subscribe(topics, (err) => {
      if (err) console.error(`Failed to subscribe v2 ${mac}`, err)
      else console.log(`Subscribed to v2 ${mac}`)
    })
  } else {
    const topicSensor = `device/ztc1/${mac}/sensor`
    const topicState = `device/ztc1/${mac}/state`
    const topicAvail = `device/ztc1/${mac}/availability`
    client.value.subscribe([topicSensor, topicState, topicAvail], (err) => {
      if (err) console.error(`Failed to subscribe ${mac}`, err)
      else console.log(`Subscribed to ${mac}`)
    })
  }
}

const handleMessage = (topic, payload) => {
    // If payload is a simpler value (like availability), handle it
    if (typeof payload !== 'object') {
       // Check if topic is availability
       if(topic.endsWith('/availability')) {
           const parts = topic.split('/')
           const mac = parts[2] // device/ztc1/MAC/availability
           if(!devices[mac]) devices[mac] = { mac }
           devices[mac].available = (payload == 1) // Loose equal as it might be number or string
           return
       }
    }

  // Determine MAC
  // Some payloads might not have MAC inside if they are defined by topic
  // But our spec says JSON payloads usually have it.
  // However, state topic from yaml example: value_template: '{{ value_json.plug_0.on }}'
  // implies payload is json. The `payload_on` includes mac.
  // Let's assume the device state reply includes mac as per original docs.
  
  let mac = payload.mac
  if (!mac) {
      // Try to extract from topic if payload doesn't have it
      const parts = topic.split('/')
      if(parts[0] === 'device' && parts[1] === 'ztc1' && parts[2]) {
          mac = parts[2]
      }
  }
  
  if (!mac) return

  if (!devices[mac]) {
    devices[mac] = { mac }
  }

  // Merge state
  Object.assign(devices[mac], payload)
}

const handleV2Message = (topic, msgStr) => {
  const parts = topic.split('/')
  if (parts.length < 5) return
  
  const [, type, mac, entity, suffix] = parts
  
  if (!devices[mac]) {
    devices[mac] = { mac, version: 'v2' }
  }
  if (!devices[mac].version) devices[mac].version = 'v2'
  
  if (suffix === 'config') {
    try {
      const config = JSON.parse(msgStr)
      if (config.device?.name) {
        devices[mac].deviceName = config.device.name
      }
      if (config.name) {
        if (!devices[mac].entityNames) devices[mac].entityNames = {}
        devices[mac].entityNames[entity] = config.name
      }
    } catch (e) {
      console.warn('Failed to parse v2 config', e)
    }
  } else if (suffix === 'state') {
    if (type === 'sensor') {
      try {
        const data = JSON.parse(msgStr)
        Object.assign(devices[mac], data)
      } catch (e) {
        console.warn('Failed to parse v2 sensor', e)
      }
    } else if (type === 'switch') {
      parseV2SwitchState(mac, msgStr)
    }
  }
}

const parseV2SwitchState = (mac, msgStr) => {
  const parts = msgStr.trim().split(' ')
  if (parts[0] !== 'set' || parts.length < 4) return
  
  const entity = parts[1]
  
  if (entity === 'socket' && parts.length >= 5) {
    const socketIndex = parseInt(parts[3])
    const state = parseInt(parts[4])
    devices[mac][`socket_${socketIndex}`] = state
  } else if (parts.length >= 4) {
    const state = parseInt(parts[3])
    devices[mac][entity] = state
  }
}

const addDevice = () => {
  let mac = newMacInput.value.trim()
  if (!mac) return
  mac = newDeviceVersion.value === 'v2' ? mac.toUpperCase() : mac.toLowerCase()
  
  if (!deviceList.value.find(d => d.mac === mac)) {
    const entry = { mac, version: newDeviceVersion.value }
    deviceList.value.push(entry)
    if (!devices[mac]) devices[mac] = { mac }
    devices[mac].version = newDeviceVersion.value
    subscribeDevice(entry)
    if (deviceList.value.length === 1) {
      selectedMac.value = mac
    }
  }
  newMacInput.value = ''
}

const removeDevice = (mac) => {
  const idx = deviceList.value.findIndex(d => d.mac === mac)
  if (idx > -1) {
    deviceList.value.splice(idx, 1)
  }
  if (selectedMac.value === mac) {
    selectedMac.value = deviceList.value[0] || null
  }
  // Optional: Unsubscribe
}

const selectDevice = (mac) => {
  selectedMac.value = mac
}

const sendCommand = (cmd) => {
    if (!client.value || !connected.value) return
    const mac = cmd.mac
    const topic = `device/ztc1/${mac}/set`
    const version = getDeviceVersion(mac)
    
    if (version === 'v2' && cmd.payload) {
      console.log('Sending v2', topic, cmd.payload)
      client.value.publish(topic, cmd.payload)
    } else {
      const payload = JSON.stringify({ ...cmd, mac })
      console.log('Sending', topic, payload)
      client.value.publish(topic, payload)
    }
}

const selectedDeviceState = computed(() => {
  if (!selectedMac.value) return null
  const state = devices[selectedMac.value] || { mac: selectedMac.value }
  const entry = deviceList.value.find(d => d.mac === selectedMac.value)
  if (entry) state.version = entry.version
  return state
})

const sidebarCollapsed = ref(false)
const toggleSidebar = () => {
    sidebarCollapsed.value = !sidebarCollapsed.value
}

const importExportRef = ref(null)
const openImportExport = () => {
    importExportRef.value?.open()
}

// Theme: 'auto' | 'light' | 'dark'
const theme = ref(localStorage.getItem('ztc1_theme') || 'auto')

const themeIcon = computed(() => {
  if (theme.value === 'dark') return '🌙'
  if (theme.value === 'light') return '☀️'
  return '🌗'
})

const themeTitle = computed(() => {
  if (theme.value === 'dark') return '深色模式 (点击切换)'
  if (theme.value === 'light') return '浅色模式 (点击切换)'
  return '跟随系统 (点击切换)'
})

const applyTheme = (t) => {
  const html = document.documentElement
  html.classList.remove('light', 'dark')
  if (t === 'light') html.classList.add('light')
  else if (t === 'dark') html.classList.add('dark')
  // 'auto' = no class, media query takes over
}

const cycleTheme = () => {
  const order = ['auto', 'dark', 'light']
  const idx = order.indexOf(theme.value)
  theme.value = order[(idx + 1) % 3]
  localStorage.setItem('ztc1_theme', theme.value)
  applyTheme(theme.value)
}

// Apply saved theme on load
applyTheme(theme.value)

</script>

<template>
  <div class="container">
    <div class="header-bar">
        <h1>zTC1 管理终端</h1>
        <div class="header-actions">
          <router-link to="/register" class="nav-link">注册</router-link>
          <router-link to="/admin" class="nav-link">管理</router-link>
          <button class="theme-btn" @click="cycleTheme" :title="themeTitle">{{ themeIcon }}</button>
          <button class="settings-btn" @click="openImportExport" title="配置迁移">⚙️</button>
        </div>
    </div>
    
    <ImportExport ref="importExportRef" />
    
    <MqttConfig 
      :connected="connected"
      @connect="connectMqtt"
      @disconnect="disconnectMqtt"
    />

    <main v-if="connected" class="main-content">
      <div class="sidebar" :class="{ collapsed: sidebarCollapsed }">
        <div class="sidebar-header">
           <span v-if="!sidebarCollapsed">设备列表</span>
           <button class="toggle-sidebar-btn" @click="toggleSidebar">
             {{ sidebarCollapsed ? '»' : '«' }}
           </button>
        </div>
        
        <div class="add-device" v-if="!sidebarCollapsed">
          <input v-model="newMacInput" placeholder="输入设备MAC地址" @keyup.enter="addDevice">
          <select v-model="newDeviceVersion" class="version-select" title="主题版本">
            <option value="v1">v1</option>
            <option value="v2">v2</option>
          </select>
          <button @click="addDevice">+</button>
        </div>
        
        <ul class="device-list" v-if="!sidebarCollapsed">
          <li 
            v-for="entry in deviceList" 
            :key="entry.mac"
            :class="{ active: entry.mac === selectedMac }"
            @click="selectDevice(entry.mac)"
          >
            <div class="device-item-row">
              <span class="device-name">
                {{ devices[entry.mac]?.deviceName || devices[entry.mac]?.name || entry.mac }}
              </span>
              <span class="version-badge">{{ entry.version }}</span>
              <button class="del-btn" @click.stop="removeDevice(entry.mac)">×</button>
            </div>
            <div class="device-status">
               {{ (devices[entry.mac]?.available === false) ? '离线' : (devices[entry.mac]?.power ? `${devices[entry.mac].power}W` : '在线') }}
            </div>
          </li>
        </ul>
        <div v-else class="collapsed-list">
            <div 
                v-for="entry in deviceList" 
                :key="entry.mac"
                class="collapsed-item"
                :class="{ active: entry.mac === selectedMac }"
                @click="selectDevice(entry.mac)"
                :title="devices[entry.mac]?.deviceName || devices[entry.mac]?.name || entry.mac"
            >
                {{ (devices[entry.mac]?.deviceName || devices[entry.mac]?.name || entry.mac).slice(-2) }}
            </div>
        </div>
      </div>

      <div class="content-area">
        <DeviceControl 
          v-if="selectedDeviceState" 
          :device="selectedDeviceState" 
          @send-cmd="sendCommand"
        />
        <div v-else class="empty-state">
          请选择或添加设备
        </div>
      </div>
    </main>
    
    <div v-else class="welcome">
      <p>请先连接 MQTT 服务器</p>
    </div>
  </div>
</template>

<style>
/* Global Styles */
*, *::before, *::after {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: var(--bg-body);
  color: var(--text-primary);
}
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 20px;
  overflow-x: hidden; /* Prevent page from being stretched */
}
.header-bar {
    position: relative;
    display: flex;
    justify-content: center;
    align-items: center;
    margin-bottom: 20px;
}
.header-actions {
    position: absolute;
    right: 0;
    display: flex;
    gap: 4px;
    align-items: center;
    z-index: 1;
}
.settings-btn, .theme-btn {
    font-size: 1.5em;
    background: none;
    border: none;
    cursor: pointer;
    padding: 5px;
    color: var(--text-primary);
    line-height: 1;
    opacity: 0.8;
}
.settings-btn:hover, .theme-btn:hover {
    opacity: 1;
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
h1 {
  text-align: center;
  color: var(--text-heading);
  margin: 0;
}

.main-content {
  display: flex;
  gap: 20px;
  min-height: 500px;
}

.sidebar {
  width: 300px;
  background: var(--bg-card);
  border-radius: 8px;
  padding: 15px;
  box-shadow: 0 2px 4px var(--shadow-color);
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
  overflow: hidden;
  height: fit-content;
  max-height: 80vh;
}

.sidebar.collapsed {
    width: 50px;
    padding: 15px 5px;
}

/* Mobile Adaptation */
@media (max-width: 768px) {
  .container {
    padding: 10px;
  }
  
  .main-content {
    flex-direction: column;
  }
  
  .sidebar {
    width: 100%;
    order: -1; /* Sidebar on top */
    max-height: 300px; /* Limit height on mobile */
  }
  
  .sidebar.collapsed {
    width: 100%;
    height: 50px;
    padding: 10px;
    flex-direction: row;
    align-items: center;
    overflow-x: auto;
    overflow-y: hidden;
  }
  
  .sidebar.collapsed .sidebar-header {
      margin-bottom: 0;
      margin-right: 10px;
  }
  
  .collapsed-list {
      flex-direction: row;
      flex-wrap: nowrap;
      overflow-x: auto;
      width: 100%;
      padding-bottom: 5px; /* Scrollbar space */
  }
  
  .collapsed-item {
      flex-shrink: 0;
  }
}

.sidebar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
    min-height: 24px;
}
.toggle-sidebar-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 1.2em;
    padding: 0 5px;
    margin: 0 auto;
    color: var(--text-primary);
}
.sidebar-header span {
    font-weight: bold;
}

.collapsed-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    align-items: center;
}
.collapsed-item {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    background: var(--bg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8em;
    cursor: pointer;
    border: 2px solid transparent;
}
.collapsed-item.active {
    background: var(--bg-active);
    border-color: #2196f3;
}

.content-area {
  flex: 1;
  background: var(--bg-card);
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 4px var(--shadow-color);
}

.add-device {
  display: flex;
  margin-bottom: 15px;
  min-width: 0; /* Prevent overflow */
}
.add-device input {
  flex: 1;
  min-width: 0; /* Allow input to shrink */
  padding: 8px;
  border: 1px solid var(--border-color);
  border-right: none;
  border-radius: 4px 0 0 4px;
  background: var(--bg-input);
  color: var(--text-primary);
}
.add-device button {
  padding: 8px 15px;
  background: #4caf50;
  color: white;
  border: none;
  border-radius: 0 4px 4px 0;
  cursor: pointer;
}
.version-select {
  padding: 8px 4px;
  border: 1px solid var(--border-color);
  border-left: none;
  background: var(--bg-secondary);
  color: var(--text-primary);
  font-size: 0.85em;
  cursor: pointer;
}
.version-badge {
  font-size: 0.7em;
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg-secondary);
  color: var(--text-secondary);
  margin-left: 4px;
  flex-shrink: 0;
}

.device-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  overflow-x: hidden; /* Prevent horizontal scroll */
  flex: 1; /* Take remaining space */
  min-height: 0; /* Allow shrinking below content size */
}
.device-list li {
  padding: 10px;
  border-bottom: 1px solid var(--border-light);
  cursor: pointer;
  transition: background 0.2s;
}
.device-list li:hover {
  background: var(--bg-hover);
}
.device-list li.active {
  background: var(--bg-active);
  border-left: 3px solid #2196f3;
}

.device-item-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-width: 0; /* Allow flex item to shrink below content size */
}
.device-name {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0; /* Allow text truncation to work */
}
.device-status {
  font-size: 0.8em;
  color: var(--text-muted);
  margin-top: 4px;
}

.del-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1.2em;
  padding: 0 5px;
}
.del-btn:hover {
  color: #f44336;
}

.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  color: var(--text-muted);
  font-size: 1.2em;
}
.welcome {
  text-align: center;
  margin-top: 50px;
  color: var(--text-secondary);
}
</style>
