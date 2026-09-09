<script setup lang="ts">
import type { Family, User } from '../types'

defineProps<{
  page: string
  menuOpen: boolean
  navItems: { key: string; label: string; icon: string }[]
  families: Family[]
  activeFamily: Family | null
  user: User | null
}>()

const emit = defineEmits<{
  navigate: [key: string]
  'update:menuOpen': [value: boolean]
  familyChange: [id: string]
  createFamily: []
  logout: []
}>()
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" :class="{ open: menuOpen }">
      <div class="brand"><div class="brand-mark">家</div><div class="brand-copy"><strong>家</strong><span>Digital Archive</span></div></div>
      <div class="nav-label">WORKSPACE</div>
      <nav class="nav">
        <button v-for="item in navItems" :key="item.key" :class="{ active: page === item.key }" @click="emit('navigate', item.key)"><span class="nav-icon">{{ item.icon }}</span><span>{{ item.label }}</span></button>
      </nav>
      <div class="sidebar-bottom"><div class="user-mini"><div class="avatar">{{ user?.display_name?.slice(0, 1) }}</div><div><span>{{ user?.display_name }}</span><small>家族档案成员</small></div><button class="icon-action" title="退出" @click="emit('logout')">↪</button></div></div>
    </aside>
    <main class="main">
      <header class="topbar"><button class="mobile-menu" @click="emit('update:menuOpen', !menuOpen)">☰</button><div class="family-select"><label>CURRENT FAMILY</label><select :value="activeFamily?.id" @change="emit('familyChange', ($event.target as HTMLSelectElement).value)"><option v-for="family in families" :key="family.id" :value="family.id">{{ family.name }}</option></select><button class="icon-action" title="创建家族" @click="emit('createFamily')">＋</button></div><div class="top-actions"><button class="icon-action" title="通知">♧</button><button class="icon-action" title="退出" @click="emit('logout')">↪</button></div></header>
      <slot />
    </main>
    <nav class="mobile-bottom"><button v-for="item in navItems.filter((item) => ['home', 'genealogy', 'persons', 'books'].includes(item.key))" :key="item.key" :class="{ active: page === item.key }" @click="emit('navigate', item.key)"><span>{{ item.icon }}</span><small>{{ item.label }}</small></button></nav>
  </div>
</template>
