<script setup>
import { ref, watch } from 'vue'
const props = defineProps({ device: { type: Object, required: true } })
const emit = defineEmits(['send-cmd'])
const speed = ref(0)
watch(() => props.device.speed, value => { speed.value = value ?? 0 }, { immediate: true })
const send = (control, value) => emit('send-cmd', { mac: props.device.mac, control, value })
const metrics = [['temperature', '温度', '°C'], ['humidity', '湿度', '%'], ['formaldehyde', '甲醛', 'mg/m³'], ['pm25', 'PM2.5', 'µg/m³']]
const switches = [['fan', '净化器'], ['child_lock', '童锁'], ['led_always_on', '灯环常亮']]
</script>
<template>
  <div>
    <h2>{{ device.name || `${device.type.toUpperCase()} ${device.type === 'm1' ? '空气检测仪' : '空气净化器'}` }}</h2>
    <p class="status">{{ device.mac }} · {{ device.available === true ? '在线' : device.available === false ? '离线' : '等待状态' }}</p>
    <template v-if="device.type === 'm1'">
      <div class="stats">
        <div v-for="[key, label, unit] in metrics" :key="key" class="stat">
          <label>{{ label }}</label><div class="value">{{ device[key] ?? '—' }} {{ unit }}</div>
        </div>
      </div>
      <label class="control-row">屏幕亮度
        <select :value="device.brightness ?? ''" @change="send('brightness', Number($event.target.value))">
          <option value="" disabled>等待状态</option>
          <option v-for="level in [0, 1, 2, 3, 4]" :key="level" :value="level">{{ level === 0 ? '关闭屏幕' : `亮度 ${level}` }}</option>
        </select>
      </label>
    </template>
    <template v-else>
      <div class="controls">
        <button v-for="[key, label] in switches" :key="key" :class="{ active: device[key] }" @click="send(key, !device[key])">{{ label }}：{{ device[key] === undefined ? '未知' : device[key] ? '开启' : '关闭' }}</button>
        <button :class="{ active: device.light?.state === 'ON' }" @click="send('light', device.light?.state === 'ON' ? 'OFF' : 'ON')">灯环：{{ device.light?.state === undefined ? '未知' : device.light.state === 'ON' ? '开启' : '关闭' }}</button>
      </div>
      <label class="control-row">风速 {{ speed }}%
        <input v-model.number="speed" type="range" min="0" max="100" @change="send('speed', speed)">
      </label>
    </template>
  </div>
</template>
<style scoped>
h2 { margin-top: 0; color: var(--text-heading); }
.status, .stat label { color: var(--text-secondary); font-size: 0.9em; }
.stats, .controls { display: flex; flex-wrap: wrap; gap: 15px; margin-bottom: 20px; }
.stat { flex: 1; min-width: 120px; padding: 15px; border-radius: 8px; background: var(--bg-secondary); }
.value { font-size: 1.2em; font-weight: bold; margin-top: 6px; }
.control-row { display: flex; align-items: center; flex-wrap: wrap; gap: 15px; margin-top: 20px; }
select, button { padding: 8px 12px; border-radius: 16px; border: 1px solid var(--border-color); background: var(--bg-secondary); color: var(--text-primary); cursor: pointer; }
button.active { background: var(--chip-active-bg); color: #1976d2; border-color: #1976d2; }
input[type=range] { flex: 1; min-width: 150px; }
</style>
