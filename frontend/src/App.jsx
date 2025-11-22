import React, { useEffect, useMemo, useRef, useState } from 'react'
import { AuthProvider, useAuth } from './AuthContext.jsx'
import { fetchTopDrugs, fetchPrescriptions, createPrescription, fetchPatientsForPhysician, fetchPhysiciansForPatient } from './dataService.js'

function Login() {
  const { login, isAuthed } = useAuth()
  const [role, setRole] = useState('admin')
  const [userId, setUserId] = useState('')
  const [error, setError] = useState('')
  const errorRef = useRef(null)

  useEffect(() => { if (error && errorRef.current) errorRef.current.focus() }, [error])
  
  // Reset userId when switching to admin
  useEffect(() => {
    if (role === 'admin') setUserId('')
  }, [role])

  function onSubmit(e) {
    e.preventDefault()
    setError('')
    if (!role) { setError('Please select a role.'); return }
    if (role !== 'admin') {
      const idNum = Number(userId)
      if (!Number.isInteger(idNum) || idNum <= 0) { setError('Enter a valid numeric user ID.'); return }
      login(role, idNum)
    } else {
      login(role, null)
    }
  }

  if (isAuthed) return null

  return (
    <div style={{maxWidth: 420, margin: '3rem auto', padding: '1rem', border: '1px solid #ddd', borderRadius: 8}}>
      <h1>HealthCarePortal</h1>
      <h2>Login</h2>
      {error && (
        <div tabIndex={-1} ref={errorRef} role="alert" aria-live="assertive" style={{color: '#b00', marginBottom: 12}}>
          {error}
        </div>
      )}
      <form onSubmit={onSubmit}>
        <label htmlFor="role">Role</label><br />
        <select id="role" aria-label="Select role" value={role} onChange={e => setRole(e.target.value)} style={{width:'100%', padding: 8, marginBottom: 12}}>
          <option value="admin">Admin</option>
          <option value="physician">Physician</option>
          <option value="patient">Patient</option>
        </select>

        {role !== 'admin' && (
          <>
            <label htmlFor="userId">{role === 'patient' ? 'Patient' : 'Physician'} ID</label><br />
            <input id="userId" aria-label="Enter your numeric ID"
                   type="number" min={1} value={userId}
                   onChange={e => setUserId(e.target.value)}
                   style={{width:'100%', padding: 8, marginBottom: 12}} />
          </>
        )}

        <button type="submit" style={{padding:'8px 14px'}}>Continue</button>
      </form>
    </div>
  )
}

function TopDrugs({ refreshKey = 0 }) {
  const { role, userId, logout } = useAuth()
  const today = useMemo(() => new Date(), [])
  const [from, setFrom] = useState(() => {
    const y = new Date().getFullYear()
    const d = new Date(Date.UTC(y, 0, 1)) // Jan 1 of current year UTC
    return d.toISOString().slice(0,10)
  })
  const [to, setTo] = useState(() => today.toISOString().slice(0,10))
  const [limit, setLimit] = useState(10)
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  // For patients: show their linked physicians on the Top Drugs screen too
  const [physicians, setPhysicians] = useState([])
  const [phLoading, setPhLoading] = useState(false)
  const [phError, setPhError] = useState('')

  async function load() {
    setLoading(true); setError('')
    try {
      const data = await fetchTopDrugs({ from, to, limit, role, userId })
      setItems(data)
    } catch (e) {
      setError(e.message || 'Failed to load data')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() // eslint-disable-next-line
  }, [])

  // When refreshKey changes (e.g., after a successful create), refetch
  useEffect(() => {
    if (refreshKey > 0) {
      load()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshKey])

  // Load patient-linked physicians (only for patient role)
  useEffect(() => {
    async function loadPhysicians() {
      if (role !== 'patient' || !userId) { setPhysicians([]); return }
      setPhLoading(true); setPhError('')
      try {
        const list = await fetchPhysiciansForPatient({ role, userId })
        setPhysicians(list)
      } catch (e) {
        setPhError(e.message || 'Failed to load physicians')
      } finally {
        setPhLoading(false)
      }
    }
    loadPhysicians()
  }, [role, userId])

  const maxQty = Math.max(1, ...items.map(i => i.total_quantity))

  return (
    <div style={{maxWidth: 900, margin: '2rem auto', padding: '1rem'}}> 
      <header style={{display:'flex', justifyContent:'space-between', alignItems:'center'}}>
        <h2>Top Drugs Report</h2>
        <div>
          <span style={{marginRight: 12}}>Role: <b>{role}</b> · User: <b>{role === 'admin' ? 'All' : userId}</b></span>
          <button onClick={logout} aria-label="Logout">Logout</button>
        </div>
      </header>

      <section aria-label="Filters" style={{display:'flex', gap:12, flexWrap:'wrap', margin:'12px 0'}}>
        <label>From <input type="date" value={from} onChange={e=>setFrom(e.target.value)} /></label>
        <label>To <input type="date" value={to} onChange={e=>setTo(e.target.value)} /></label>
        <label>Limit <input type="number" min={1} max={100} value={limit} onChange={e=>setLimit(Number(e.target.value))} style={{width:80}}/></label>
        <button onClick={load} disabled={loading}>Apply</button>
      </section>

      {role === 'patient' && (
        <section aria-label="My physicians" style={{margin:'0 0 12px'}}>
          <strong>My Physicians:</strong>{' '}
          <span aria-live="polite" style={{color:'#555'}}>
            {phLoading && 'Loading…'}
            {phError && <span style={{color:'#b00'}}>Error: {phError}</span>}
            {!phLoading && !phError && physicians.length === 0 && 'None linked.'}
            {!phLoading && !phError && physicians.length > 0 && physicians.map((ph, i) => (
              <span key={ph.id}>{i>0 && ', '}{ph.name} (#{ph.id})</span>
            ))}
          </span>
        </section>
      )}

      <div aria-live="polite" style={{minHeight: 24, color: '#555'}}>
        {loading && <span>Loading…</span>}
        {error && <span style={{color:'#b00'}}>Error: {error}</span>}
        {!loading && !error && items.length === 0 && <span>No data for selected filters.</span>}
      </div>

      {items.length > 0 && (
        <div style={{display:'grid', gridTemplateColumns: '1fr 1fr', gap: 16}}>
          <table style={{width:'100%', borderCollapse:'collapse'}} aria-label="Top drugs table">
            <thead>
              <tr>
                <th scope="col" style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>#</th>
                <th scope="col" style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>Drug</th>
                <th scope="col" style={{textAlign:'right', borderBottom:'1px solid #ccc'}}>Total Qty</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it, idx) => (
                <tr key={it.drug_id}>
                  <td>{idx+1}</td>
                  <td>{it.drug_name}</td>
                  <td style={{textAlign:'right'}}>{it.total_quantity}</td>
                </tr>
              ))}
            </tbody>
          </table>

          <figure aria-label="Top drugs bar chart" style={{border:'1px solid #eee', padding: 8}}>
            <svg width="100%" height={items.length * 34 + 40} role="img" aria-labelledby="chart-title">
              <title id="chart-title">Top drugs by total quantity</title>
              {items.map((it, idx) => {
                const w = (it.total_quantity / maxQty) * 300
                const y = idx * 34 + 20
                return (
                  <g key={it.drug_id} transform={`translate(140, ${y})`}>
                    <text x={-10} y={14} textAnchor="end" style={{fontSize:12}}>{it.drug_name}</text>
                    <rect x={0} y={0} width={w} height={20} fill="#4f46e5" />
                    <text x={w + 6} y={14} style={{fontSize:12}}>{it.total_quantity}</text>
                  </g>
                )
              })}
            </svg>
            <figcaption style={{fontSize:12, color:'#555'}}>Each bar shows total quantity for the drug within the selected date range.</figcaption>
          </figure>
        </div>
      )}
    </div>
  )
}

function PrescriptionsList() {
  const { role, userId } = useAuth()
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  // For patients: show their linked physicians
  const [physicians, setPhysicians] = useState([])
  const [phLoading, setPhLoading] = useState(false)
  const [phError, setPhError] = useState('')

  async function load() {
    setLoading(true); setError('')
    try {
      const data = await fetchPrescriptions({ role, userId, limit: 50 })
      setItems(data)
    } catch (e) {
      setError(e.message || 'Failed to load prescriptions')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() // eslint-disable-next-line
  }, [])

  // Load patient-linked physicians on mount (only for patient role)
  useEffect(() => {
    async function loadPhys() {
      if (role !== 'patient' || !userId) { setPhysicians([]); return }
      setPhLoading(true); setPhError('')
      try {
        const data = await fetchPhysiciansForPatient({ role, userId })
        setPhysicians(data)
      } catch (e) {
        setPhError(e.message || 'Failed to load physicians')
      } finally {
        setPhLoading(false)
      }
    }
    loadPhys()
  }, [role, userId])

  return (
    <div style={{maxWidth: 900, margin: '2rem auto', padding: '1rem'}}>
      <h2>My Prescriptions</h2>
      {role === 'patient' && (
        <section aria-label="My physicians" style={{margin:'8px 0 16px'}}>
          <strong>My Physicians:</strong>{' '}
          <span aria-live="polite" style={{color:'#555'}}>
            {phLoading && 'Loading…'}
            {phError && <span style={{color:'#b00'}}>Error: {phError}</span>}
            {!phLoading && !phError && physicians.length === 0 && 'None linked.'}
            {!phLoading && !phError && physicians.length > 0 && physicians.map((ph, i) => (
              <span key={ph.id}>
                {i>0 && ', '}{ph.name} (#{ph.id})
              </span>
            ))}
          </span>
        </section>
      )}
      <div aria-live="polite" style={{minHeight: 24, color: '#555'}}>
        {loading && <span>Loading…</span>}
        {error && <span style={{color:'#b00'}}>Error: {error}</span>}
        {!loading && !error && items.length === 0 && <span>No prescriptions.</span>}
      </div>

      {items.length > 0 && (
        <table style={{width:'100%', borderCollapse:'collapse'}} aria-label="Prescriptions table">
          <thead>
            <tr>
              <th style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>ID</th>
              <th style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>Patient</th>
              <th style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>Physician</th>
              <th style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>Drug</th>
              <th style={{textAlign:'right', borderBottom:'1px solid #ccc'}}>Qty</th>
              <th style={{textAlign:'left', borderBottom:'1px solid #ccc'}}>When</th>
            </tr>
          </thead>
          <tbody>
            {items.map(p => (
              <tr key={p.id}>
                <td>{p.id}</td>
                <td>{p.patient_name || `Patient #${p.patient_id}`}</td>
                <td>{p.physician_name || `Physician #${p.physician_id}`}</td>
                <td>{p.drug_name || `Drug #${p.drug_id}`}</td>
                <td style={{textAlign:'right'}}>{p.quantity}</td>
                <td>{new Date(p.prescribed_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function CreatePrescriptionForm({ onCreated }) {
  const { role, userId } = useAuth()
  const [form, setForm] = useState({ patient_id: '', physician_id: '', drug_name: '', quantity: 1, sig: '' })
  const [msg, setMsg] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [patients, setPatients] = useState([])
  const [pLoading, setPLoading] = useState(false)
  const [pError, setPError] = useState('')

  useEffect(() => {
    if (role === 'physician') setForm(f => ({ ...f, physician_id: userId }))
  }, [role, userId])

  // Load linked patients when physician logs in or changes
  useEffect(() => {
    async function loadPatients() {
      if (role !== 'physician' || !userId) { setPatients([]); return }
      setPLoading(true); setPError('')
      try {
        const items = await fetchPatientsForPhysician({ role, userId })
        setPatients(items)
        // preset patient if none selected
        if (items.length > 0) {
          setForm(f => ({ ...f, patient_id: f.patient_id || items[0].id }))
        }
      } catch (e) {
        setPError(e.message || 'Failed to load patients')
      } finally {
        setPLoading(false)
      }
    }
    loadPatients()
  }, [role, userId])

  async function onSubmit(e) {
    e.preventDefault()
    setMsg(''); setError('')
    if (role !== 'physician') { setError('Only physicians may create prescriptions'); return }
    const payload = {
      patient_id: Number(form.patient_id),
      physician_id: Number(form.physician_id),
      drug_id: 0,
      drug_name: String(form.drug_name || '').trim(),
      quantity: Number(form.quantity),
      sig: String(form.sig||'').trim(),
    }
    setBusy(true)
    try {
      const created = await createPrescription({ role, userId, payload })
      setMsg(`Created prescription #${created.id}`)
      if (typeof onCreated === 'function') {
        onCreated(created)
      }
    } catch (e) {
      setError(e.message || 'Failed to create')
    } finally {
      setBusy(false)
    }
  }

  const disabled = role !== 'physician'

  return (
    <div style={{maxWidth: 600, margin: '2rem auto', padding: '1rem'}}>
      <h2>Create Prescription</h2>
      <div aria-live="polite" style={{minHeight: 24}}>
        {msg && <span style={{color:'#065f46'}}>{msg}</span>}
        {error && <span style={{color:'#b00'}}>{error}</span>}
      </div>
      <form onSubmit={onSubmit} aria-label="Create prescription form">
        <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap:12}}>
          {role === 'physician' ? (
            <label>Patient
              <select required value={form.patient_id} onChange={e=>setForm({...form, patient_id: Number(e.target.value)})} disabled={disabled}>
                {patients.map(p => (
                  <option key={p.id} value={p.id}>{p.name} (#{p.id})</option>
                ))}
              </select>
              <div aria-live="polite" style={{fontSize:12, color:'#555'}}>
                {pLoading && 'Loading patients…'}
                {pError && <span style={{color:'#b00'}}>Error: {pError}</span>}
                {!pLoading && !pError && patients.length === 0 && <span>No linked patients.</span>}
              </div>
            </label>
          ) : (
            <label>Patient ID
              <input type="number" required value={form.patient_id} onChange={e=>setForm({...form, patient_id: e.target.value})} disabled={disabled} />
            </label>
          )}
          <label>Physician ID
            <input type="number" required value={form.physician_id} onChange={e=>setForm({...form, physician_id: e.target.value})} disabled={true} />
          </label>
          <label>Drug Name
            <input type="text" required value={form.drug_name} onChange={e=>setForm({...form, drug_name: e.target.value})} disabled={disabled} placeholder="e.g., Ibuprofen" />
          </label>
          <label>Quantity
            <input type="number" min={1} required value={form.quantity} onChange={e=>setForm({...form, quantity: e.target.value})} disabled={disabled} />
          </label>
        </div>
        <label>Sig
          <input type="text" required value={form.sig} onChange={e=>setForm({...form, sig: e.target.value})} style={{width:'100%'}} disabled={disabled} />
        </label>
        <div style={{marginTop:12}}>
          <button type="submit" disabled={busy || disabled}>
            {role !== 'physician' ? 'Only physicians may create' : (busy ? 'Creating…' : 'Create')}
          </button>
        </div>
      </form>
    </div>
  )
}

function Shell() {
  const { isAuthed, role } = useAuth()
  const [tab, setTab] = useState('report') // 'report' | 'list' | 'create'
  const [analyticsNonce, setAnalyticsNonce] = useState(0)
  if (!isAuthed) return <Login />
  return (
    <div>
      <nav style={{display:'flex', gap:8, padding:'8px 12px', borderBottom:'1px solid #eee'}}>
        <button onClick={()=>setTab('report')} aria-current={tab==='report' ? 'page' : undefined}>Top Drugs</button>
        <button onClick={()=>setTab('list')} aria-current={tab==='list' ? 'page' : undefined}>My Prescriptions</button>
        {role === 'physician' && (
          <button onClick={()=>setTab('create')} aria-current={tab==='create' ? 'page' : undefined}>Create</button>
        )}
      </nav>
      {tab === 'report' && <TopDrugs refreshKey={analyticsNonce} />}
      {tab === 'list' && <PrescriptionsList />}
      {tab === 'create' && role === 'physician' && (
        <CreatePrescriptionForm onCreated={() => setAnalyticsNonce(n => n + 1)} />
      )}
    </div>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <Shell />
    </AuthProvider>
  )
}
