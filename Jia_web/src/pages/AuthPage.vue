<script setup lang="ts">
import { computed, ref, watch } from 'vue'

type AuthMode = 'login' | 'register' | 'forgot' | 'reset'

const props = defineProps<{ busy: boolean; resetToken?: string; resetRequestedAt?: number; resetCompletedAt?: number }>()
const authMode = ref<AuthMode>('login')
const auth = ref({ email: '', password: '', confirmPassword: '', displayName: '', resetToken: '' })
const formError = ref('')
const copyStatus = ref('')
const emit = defineEmits<{
  submit: [payload: { mode: AuthMode; email: string; password: string; confirmPassword: string; displayName: string; resetToken: string }]
}>()

const title = computed(() => ({ login: '进入家族档案', register: '建立你的档案', forgot: '找回家族档案', reset: '设置新的密码' })[authMode.value])
const subtitle = computed(() => ({ login: '欢迎回来，继续整理家族的故事。', register: '从一个名字开始，邀请家人一起加入。', forgot: '输入注册邮箱，获取一次性重置凭证。', reset: '设置新密码后即可重新登录。' })[authMode.value])
const submitLabel = computed(() => ({ login: '登录档案', register: '创建账号', forgot: '获取重置凭证', reset: '保存新密码' })[authMode.value])

watch(() => props.resetRequestedAt, (value) => {
  if (value) {
    auth.value.password = ''
    auth.value.confirmPassword = ''
    formError.value = ''
    copyStatus.value = ''
    authMode.value = 'reset'
  }
})
watch(() => props.resetCompletedAt, (value) => {
  if (value) {
    authMode.value = 'login'
    auth.value.password = ''
    auth.value.confirmPassword = ''
    auth.value.resetToken = ''
    formError.value = ''
    copyStatus.value = ''
  }
})

function submitForm() {
  formError.value = ''
  if (authMode.value === 'reset' && auth.value.password !== auth.value.confirmPassword) {
    formError.value = '两次输入的密码不一致'
    return
  }
  emit('submit', { mode: authMode.value, ...auth.value, resetToken: props.resetToken || auth.value.resetToken })
}
function switchMode(mode: AuthMode) {
  authMode.value = mode
  formError.value = ''
  copyStatus.value = ''
  if (mode === 'login' || mode === 'forgot') auth.value.password = ''
}
async function copyResetToken() {
  const token = props.resetToken || auth.value.resetToken
  if (!token) return
  try {
    await navigator.clipboard.writeText(token)
    copyStatus.value = '凭证已复制'
  } catch {
    copyStatus.value = '复制失败，请手动复制凭证'
  }
}
</script>

<template>
  <div class="auth-screen">
    <section class="auth-art"><div class="brand"><div class="brand-mark">家</div><div class="brand-copy"><strong>家</strong><span>Digital Family Archive</span></div></div><h1>把家，<br>留给时间。</h1><p>一份由全家共同书写的数字档案，记录我们的来处，也让未来的人知道我们是谁。</p></section>
    <section class="auth-form"><div class="auth-box"><h2>{{ title }}</h2><p>{{ subtitle }}</p><form class="form-grid" @submit.prevent="submitForm">
      <label v-if="authMode === 'register'">你的称呼<input v-model="auth.displayName" placeholder="例如：张三" required></label>
      <label v-if="authMode !== 'reset'">邮箱地址<input v-model="auth.email" type="email" placeholder="name@example.com" required></label>
      <label v-if="authMode === 'reset'">重置凭证<input :value="props.resetToken || auth.resetToken" placeholder="请粘贴收到的凭证" required autocomplete="one-time-code" @input="auth.resetToken = ($event.target as HTMLInputElement).value"><a-button v-if="props.resetToken || auth.resetToken" type="text" size="small" html-type="button" @click="copyResetToken">复制凭证</a-button></label>
      <label v-if="authMode === 'login' || authMode === 'register' || authMode === 'reset'">{{ authMode === 'reset' ? '新密码' : '密码' }}<input v-model="auth.password" type="password" placeholder="至少六位密码" minlength="6" required autocomplete="new-password"></label>
      <label v-if="authMode === 'reset'">确认新密码<input v-model="auth.confirmPassword" type="password" placeholder="再次输入新密码" minlength="6" required autocomplete="new-password"></label>
      <p v-if="formError" class="form-error">{{ formError }}</p>
      <p v-if="copyStatus" class="form-hint">{{ copyStatus }}</p>
      <a-button html-type="submit" type="primary" class="primary" :loading="props.busy">{{ submitLabel }}</a-button>
    </form><div v-if="authMode === 'login'" class="forgot-link"><button type="button" @click="switchMode('forgot')">忘记密码？</button></div><div class="switch-auth"><template v-if="authMode === 'login'">还没有账号？<button type="button" @click="switchMode('register')">立即注册</button></template><template v-else-if="authMode === 'register'">已经有账号？<button type="button" @click="switchMode('login')">返回登录</button></template><template v-else><button type="button" @click="switchMode('login')">返回登录</button></template></div></div></section>
  </div>
</template>
