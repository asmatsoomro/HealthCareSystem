// Frontend always calls the real backend API
const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export async function fetchTopDrugs({ from, to, limit = 10, role, userId }) {
  const url = new URL(`${API_BASE}/analytics/top-drugs`)
  // Use full-day RFC3339 bounds for backend
  const fromISO = new Date(from + 'T00:00:00Z').toISOString()
  const toISO = new Date(to + 'T23:59:59Z').toISOString()
  url.searchParams.set('from', fromISO)
  url.searchParams.set('to', toISO)
  url.searchParams.set('limit', String(limit))
  // Cache-busting to ensure fresh analytics after creation
  url.searchParams.set('_', String(Date.now()))
  const headers = new Headers({ 'X-Role': role })
  if (role !== 'admin' && userId != null) headers.set('X-User-ID', String(userId))
  let res
  try {
    res = await fetch(url.toString(), { headers, cache: 'no-store' })
  } catch (e) {
    throw new Error('Network error: unable to reach API (check docker compose up, ports, and CORS)')
  }
  if (!res.ok) {
    let msg = `Backend error: ${res.status}`
    try { const j = await res.json(); if (j && j.error) msg = j.error } catch {}
    throw new Error(msg)
  }
  const body = await res.json()
  return body.items || []
}

export async function fetchPrescriptions({ role, userId, limit = 50, filters = {} }) {
  const url = new URL(`${API_BASE}/prescriptions`)
  url.searchParams.set('limit', String(limit))
  if (role === 'admin') {
    if (filters.patient_id) url.searchParams.set('patient_id', String(filters.patient_id))
    if (filters.physician_id) url.searchParams.set('physician_id', String(filters.physician_id))
  }
  const headers = new Headers({ 'X-Role': role })
  if (role !== 'admin' && userId != null) headers.set('X-User-ID', String(userId))
  let res
  try { res = await fetch(url.toString(), { headers }) } catch (e) { throw new Error('Network error: unable to reach API') }
  if (!res.ok) {
    let msg = `Backend error: ${res.status}`
    try { const j = await res.json(); if (j && j.error) msg = j.error } catch {}
    throw new Error(msg)
  }
  const body = await res.json()
  return body.items || []
}

export async function createPrescription({ role, userId, payload }) {
  const headers = new Headers({ 'Content-Type': 'application/json', 'X-Role': role })
  if (role !== 'admin' && userId != null) headers.set('X-User-ID', String(userId))
  const res = await fetch(`${API_BASE}/prescriptions`, { method: 'POST', headers, body: JSON.stringify(payload) })
  const text = await res.text()
  if (!res.ok) {
    try { const j = JSON.parse(text); throw new Error(j.error || `Backend error: ${res.status}`) } catch { throw new Error(text || `Backend error: ${res.status}`) }
  }
  return JSON.parse(text)
}

// Fetch patients linked to a physician (for dropdown)
export async function fetchPatientsForPhysician({ role, userId, physicianId }) {
  const id = physicianId ?? userId
  const headers = new Headers({ 'X-Role': role })
  if (role !== 'admin' && userId != null) headers.set('X-User-ID', String(userId))
  let res
  try {
    res = await fetch(`${API_BASE}/physicians/${id}/patients`, { headers })
  } catch (e) {
    throw new Error('Network error: unable to reach API')
  }
  if (!res.ok) {
    let msg = `Backend error: ${res.status}`
    try { const j = await res.json(); if (j && j.error) msg = j.error } catch {}
    throw new Error(msg)
  }
  const body = await res.json()
  return body.items || []
}

// Fetch physicians linked to a patient (for patient dashboard)
export async function fetchPhysiciansForPatient({ role, userId, patientId }) {
  const id = patientId ?? userId
  const headers = new Headers({ 'X-Role': role })
  if (role !== 'admin' && userId != null) headers.set('X-User-ID', String(userId))
  let res
  try {
    res = await fetch(`${API_BASE}/patients/${id}/physicians`, { headers })
  } catch (e) {
    throw new Error('Network error: unable to reach API')
  }
  if (!res.ok) {
    let msg = `Backend error: ${res.status}`
    try { const j = await res.json(); if (j && j.error) msg = j.error } catch {}
    throw new Error(msg)
  }
  const body = await res.json()
  return body.items || []
}
