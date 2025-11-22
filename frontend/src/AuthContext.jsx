import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [role, setRole] = useState(null) // 'admin' | 'physician' | 'patient'
  const [userId, setUserId] = useState(null) // number | null

  // hydrate from sessionStorage
  useEffect(() => {
    try {
      const saved = JSON.parse(sessionStorage.getItem('hcp-auth') || 'null')
      if (saved && saved.role) {
        setRole(saved.role)
        // userId may be null for admin
        setUserId(saved.userId ?? null)
      }
    } catch {}
  }, [])

  useEffect(() => {
    if (role) {
      sessionStorage.setItem('hcp-auth', JSON.stringify({ role, userId }))
    } else {
      sessionStorage.removeItem('hcp-auth')
    }
  }, [role, userId])

  const value = useMemo(() => ({
    role,
    userId,
    isAuthed: !!role && (role === 'admin' || !!userId),
    login: (nextRole, nextUserId = null) => { setRole(nextRole); setUserId(nextUserId) },
    logout: () => { setRole(null); setUserId(null) }
  }), [role, userId])

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
