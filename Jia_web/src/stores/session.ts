import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi, familyApi } from '../api'
import type { Family, User } from '../types'

export const useSessionStore = defineStore('session', () => {
  const user = ref<User | null>(null)
  const families = ref<Family[]>([])
  const activeFamilyId = ref(localStorage.getItem('jia_family') || '')
  const isAuthenticated = computed(() => Boolean(user.value && localStorage.getItem('jia_token')))
  const activeFamily = computed(() => families.value.find((f) => f.id === activeFamilyId.value) || families.value[0] || null)

  async function login(email: string, password: string) {
    const result = await authApi.login({ email, password })
    localStorage.setItem('jia_token', result.token); user.value = result.user; await loadFamilies()
  }
  async function register(email: string, password: string, displayName: string) {
    const result = await authApi.register({ email, password, displayName })
    localStorage.setItem('jia_token', result.token); user.value = result.user; await loadFamilies()
  }
  async function bootstrap() {
    if (!localStorage.getItem('jia_token')) return
    try { user.value = await authApi.me(); await loadFamilies() } catch { logout() }
  }
  async function loadFamilies() { families.value = await familyApi.list(); if (!families.value.some((f) => f.id === activeFamilyId.value)) setActiveFamily(families.value[0]?.id || '') }
  function setActiveFamily(id: string) { activeFamilyId.value = id; localStorage.setItem('jia_family', id) }
  function logout() { user.value = null; families.value = []; activeFamilyId.value = ''; localStorage.removeItem('jia_token'); localStorage.removeItem('jia_family') }
  return { user, families, activeFamily, activeFamilyId, isAuthenticated, login, register, bootstrap, loadFamilies, setActiveFamily, logout }
})
