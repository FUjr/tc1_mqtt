import test from 'node:test'
import assert from 'node:assert/strict'
import { applyAirMessage, airCommand } from '../src/deviceProtocol.js'
const m1 = { type: 'm1', mac: 'aabbccddeeff' }
const a1 = { type: 'a1', mac: '112233445566' }
test('legacy telemetry, availability and malformed/untracked input', () => {
  const states = {}
  assert.equal(applyAirMessage([m1, a1], states, 'phicomm/m1/aabbccddeeff/sensor', '{"temperature":23.5,"humidity":50,"formaldehyde":0.012,"pm25":8}'), true)
  assert.equal(states[m1.mac].formaldehyde, 0.012)
  applyAirMessage([m1], states, 'phicomm/m1/aabbccddeeff/sensor', 'null')
  applyAirMessage([m1], states, 'phicomm/m1/aabbccddeeff/availability', 'offline')
  assert.equal(states[m1.mac].available, false)
  assert.equal(applyAirMessage([m1], states, 'phicomm/m1/ffffffffffff/sensor', '{}'), false)
  applyAirMessage([a1], states, 'phicomm/a1/112233445566/fan/state', 'ON')
  applyAirMessage([a1], states, 'phicomm/a1/112233445566/fan/percentage/state', '65')
  applyAirMessage([a1], states, 'phicomm/a1/112233445566/light/state', '{"state":"OFF","brightness":128}')
  assert.equal(states[a1.mac].fan, true)
  assert.equal(states[a1.mac].speed, 65)
  assert.equal(states[a1.mac].light.state, 'OFF')
})
test('firmware command topics, payloads and range boundaries', () => {
  assert.deepEqual(airCommand(m1, { control: 'brightness', value: 4 }), { topic: 'phicomm/m1/aabbccddeeff/brightness/set', payload: '4' })
  assert.equal(airCommand(m1, { control: 'brightness', value: 5 }), null)
  assert.equal(airCommand(m1, { control: 'fan', value: true }), null)
  assert.equal(airCommand(a1, { control: 'speed', value: 101 }), null)
  assert.equal(airCommand(a1, { control: 'speed', value: 0 }).payload, '0')
  assert.equal(airCommand(a1, { control: 'child_lock', value: true }).payload, 'ON')
  assert.deepEqual(airCommand(a1, { control: 'light', value: 'OFF' }), { topic: 'phicomm/a1/112233445566/light/set', payload: '{"state":"OFF"}' })
})
