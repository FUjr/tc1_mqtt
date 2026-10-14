export const legacyBase = ({ type, mac }) => `phicomm/${type}/${mac}`
export function applyAirMessage(entries, devices, topic, text) {
  const entry = entries.find(e => ['m1', 'a1'].includes(e.type) && topic.startsWith(`${legacyBase(e)}/`))
  if (!entry) return false
  const leaf = topic.slice(legacyBase(entry).length + 1)
  const state = devices[entry.mac] ||= { mac: entry.mac, type: entry.type }
  if (leaf === 'availability') state.available = text === 'online'
  else if (entry.type === 'm1' && leaf === 'sensor') {
    try {
      const data = JSON.parse(text)
      for (const key of ['temperature', 'humidity', 'formaldehyde', 'pm25']) {
        if (typeof data?.[key] === 'number' && Number.isFinite(data[key])) state[key] = data[key]
      }
    } catch { /* Ignore malformed telemetry. */ }
  } else if (entry.type === 'a1' && leaf === 'light/state') {
    try { const data = JSON.parse(text); if (data && typeof data === 'object') state.light = data } catch { /* Ignore malformed state. */ }
  } else if (['brightness/state', 'fan/percentage/state'].includes(leaf)) {
    if (text.trim() && Number.isFinite(Number(text))) state[leaf === 'brightness/state' ? 'brightness' : 'speed'] = Number(text)
  } else if (['fan/state', 'child_lock/state', 'led_always_on/state'].includes(leaf)) {
    if (['ON', 'OFF'].includes(text)) state[leaf.split('/')[0]] = text === 'ON'
  }
  return true
}
export function airCommand(entry, { control, value }) {
  const limits = entry.type === 'm1' ? { brightness: [0, 4] } : { speed: [0, 100] }
  let leaf, payload
  if (limits[control]) {
    const n = Number(value), [min, max] = limits[control]
    if (!Number.isInteger(n) || n < min || n > max) return null
    leaf = control === 'speed' ? 'fan/percentage/set' : 'brightness/set'; payload = String(n)
  } else if (entry.type === 'a1' && ['fan', 'child_lock', 'led_always_on'].includes(control) && typeof value === 'boolean') {
    leaf = `${control}/set`; payload = value ? 'ON' : 'OFF'
  } else if (entry.type === 'a1' && control === 'light' && ['ON', 'OFF'].includes(value)) {
    leaf = 'light/set'; payload = JSON.stringify({ state: value })
  } else return null
  return { topic: `${legacyBase(entry)}/${leaf}`, payload }
}
