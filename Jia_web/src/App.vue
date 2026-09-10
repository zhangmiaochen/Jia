<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Message, Notification } from '@arco-design/web-vue'
import { authApi, bookApi, familyApi, genealogyApi, personApi, photoApi } from './api'
import { useSessionStore } from './stores/session'
import AppLayout from './layouts/AppLayout.vue'
import AuthPage from './pages/AuthPage.vue'
import HomePage from './pages/HomePage.vue'
import GenealogyPage from './pages/GenealogyPage.vue'
import PersonsPage from './pages/PersonsPage.vue'
import GraphPage from './pages/GraphPage.vue'
import BooksPage from './pages/BooksPage.vue'
import PhotosPage from './pages/PhotosPage.vue'
import MembersPage from './pages/MembersPage.vue'
import FamilyPage from './pages/FamilyPage.vue'
import ActivityPage from './pages/ActivityPage.vue'
import RecordModals from './components/RecordModals.vue'
import type { Activity, Book, FamilyMember, Genealogy, GraphData, MergeSummary, Person, PersonFamily, Photo } from './types'
import { useRoute, useRouter } from 'vue-router'

const session = useSessionStore()
const route = useRoute(); const router = useRouter()
const page = ref(String(route.name || 'home')); const busy = ref(false); const menuOpen = ref(false)
const resetToken = ref(''); const resetRequestedAt = ref(0); const resetCompletedAt = ref(0)
const genealogies = ref<Genealogy[]>([]); const persons = ref<Person[]>([]); const books = ref<Book[]>([]); const activities = ref<Activity[]>([]); const members = ref<FamilyMember[]>([]); const photos = ref<Photo[]>([])
const selectedPerson = ref<Person | null>(null); const selectedPersonId = ref(''); const graph = ref<GraphData | null>(null); const query = ref('')
const personVisible = ref(false); const relationVisible = ref(false); const bookVisible = ref(false); const bookEditorVisible = ref(false); const familyVisible = ref(false); const familyEditing = ref(false); const genealogyVisible = ref(false); const memberVisible = ref(false); const photoVisible = ref(false); const moveVisible = ref(false); const moveBusy = ref(false)
const moveTarget = ref<Person | null>(null); const moveForm = ref({ familyId: '' })
const personFamilyVisible = ref(false); const personFamilyBusy = ref(false); const personFamilyTarget = ref<Person | null>(null); const personFamilyList = ref<PersonFamily[]>([]); const personFamilyForm = ref({ familyId: '' }); const personFamilies = ref<PersonFamily[]>([])
const familyForm = ref({ name: '', surname: '', origin: '', migrationHistory: '', creed: '', description: '' }); const genealogyForm = ref({ name: '', description: '' }); const memberForm = ref<{ email: string; role: 'editor' | 'viewer' }>({ email: '', role: 'editor' }); const photoForm = ref({ personId: '', caption: '', category: 'family', file: null as File | null })
const personForm = ref<{ name: string; gender: 'male' | 'female' | 'unknown'; birthDate: string; birthplace: string; occupation: string; biography: string; genealogyID: string }>({ name: '', gender: 'unknown', birthDate: '', birthplace: '', occupation: '', biography: '', genealogyID: '' })
const relationForm = ref({ toPersonID: '', relationType: 'FATHER', customName: '', note: '' }); const bookForm = ref({ title: '', rootPersonID: '', contentJSON: { type: 'doc', content: [{ type: 'paragraph' }] } }); const editingBook = ref<Book | null>(null); const bookEditorText = ref('')
const navItems = [{ key: 'home', label: '家族首页', icon: '⌂' }, { key: 'genealogy', label: '族谱', icon: '⌘' }, { key: 'persons', label: '人物', icon: '♙' }, { key: 'books', label: '家谱', icon: '▤' }, { key: 'photos', label: '照片', icon: '▧' }, { key: 'graph', label: '关系图', icon: '⌁' }, { key: 'members', label: '家族成员', icon: '♧' }, { key: 'family', label: '家族资料', icon: '▦' }, { key: 'activity', label: '动态', icon: '◷' }]
const activeFamily = computed(() => session.activeFamily)
const myRole = computed(() => members.value.find((item) => item.user_id === session.user?.id)?.role || '')
const mergeToken = ref(String(route.query.merge || ''))
const pageMeta = computed<[string, string, string]>(() => {
  const metas: Record<string, [string, string, string]> = { home: ['ARCHIVE / OVERVIEW', '家族首页', '这里是家族档案的总览。每一条记录，都在为下一代保留一份清晰的来处。'], genealogy: ['LINEAGE / RECORDS', '族谱', '用世系与关系，把散落在不同地方的家人重新连在一起。'], persons: ['PEOPLE / INDEX', '人物', '家族中的每一位成员，都值得拥有一份完整而有温度的档案。'], books: ['MEMORY / LIBRARY', '家谱', '记录人物生平、家庭故事与那些不该被遗忘的时刻。'], photos: ['PHOTOS / ALBUM', '照片', '为老照片留下名字、时间与它背后的故事。'], graph: ['RELATIONS / MAP', '关系图', '从一个人出发，查看一张家族关系的切片。'], members: ['FAMILY / COLLABORATION', '家族成员', '邀请家人共同维护这份属于全家的档案。'], family: ['FAMILY / PROFILE', '家族资料', '记录家族的姓氏、起源、迁徙与家训。'], activity: ['ARCHIVE / ACTIVITY', '家族动态', '最近发生的每一次更新，都会在这里留下痕迹。'] }
  return metas[page.value] || metas.home
})

watch(() => route.name, (name) => { if (typeof name === 'string') page.value = name })
// 合并码可以做成 /family?merge=XXXX-XXXX-XXXX 的链接发给对方：进来自动跳到家族资料页并打开合并向导。
watch(() => route.query.merge, (value) => { const token = typeof value === 'string' ? value.trim() : ''; if (!token) return; mergeToken.value = token; if (page.value !== 'family') router.push({ name: 'family' }) })
onMounted(() => { if (mergeToken.value && page.value !== 'family') router.push({ name: 'family' }) })
onMounted(() => session.bootstrap().then(() => { if (session.isAuthenticated) loadAll() }))
async function afterMerge(summary: MergeSummary) { try { await session.loadFamilies(); if (summary?.target_family?.id) session.setActiveFamily(summary.target_family.id); selectedPerson.value = null; selectedPersonId.value = ''; graph.value = null; await loadAll(); Message.success(`已切换到「${summary?.target_family?.name || '合并后的家族'}」`) } catch (error: any) { Message.error(error.message) } }
async function loadAll() { if (!activeFamily.value) return; busy.value = true; try { [genealogies.value, persons.value, books.value, activities.value, members.value, photos.value] = await Promise.all([genealogyApi.list(activeFamily.value.id), personApi.list(activeFamily.value.id), bookApi.list(activeFamily.value.id), familyApi.activities(activeFamily.value.id), familyApi.members(activeFamily.value.id), photoApi.list(activeFamily.value.id)]); graph.value = await familyApi.graph(activeFamily.value.id, selectedPersonId.value) } catch (error: any) { Message.error(error.message) } finally { busy.value = false } }
async function submitAuth(payload: { mode: 'login' | 'register' | 'forgot' | 'reset'; email: string; password: string; confirmPassword: string; displayName: string; resetToken: string }) {
  busy.value = true
  try {
    if (payload.mode === 'login') {
      await session.login(payload.email, payload.password)
      await loadAll()
      Notification.success({ title: '欢迎回来', content: '家族档案已经准备好。' })
    } else if (payload.mode === 'register') {
      await session.register(payload.email, payload.password, payload.displayName)
      await loadAll()
      Notification.success({ title: '账号已创建', content: '家族档案已经准备好。' })
    } else if (payload.mode === 'forgot') {
      const result = await authApi.forgotPassword(payload.email)
      resetToken.value = String(result.reset_token || '').trim()
      if (!resetToken.value) {
        Message.error('未获取到重置凭证，请确认邮箱已注册并完成数据库迁移')
        return
      }
      resetRequestedAt.value = Date.now()
      if (resetToken.value) {
        try {
          await navigator.clipboard.writeText(resetToken.value)
        } catch {
          Message.info('凭证已填入页面，请手动复制')
        }
      }
      Message.success(result.message)
    } else {
      await authApi.resetPassword(payload.resetToken, payload.password)
      resetToken.value = ''
      resetCompletedAt.value = Date.now()
      Message.success('密码已重置，请使用新密码登录')
    }
  } catch (error: any) {
    Message.error(error.message)
  } finally {
    busy.value = false
  }
}
function go(key: string) { menuOpen.value = false; if (page.value !== key) router.push({ name: key }); if (key === 'graph' && persons.value[0]) openGraph(persons.value.find((item) => item.id === selectedPersonId.value) || persons.value[0]) }
async function changeFamily(id: string) { session.setActiveFamily(id); selectedPerson.value = null; selectedPersonId.value = ''; graph.value = null; await loadAll() }
function logout() { session.logout(); page.value = 'home' }
function createFamily() { familyEditing.value = false; familyForm.value = { name: '', surname: '', origin: '', migrationHistory: '', creed: '', description: '' }; familyVisible.value = true }
function editFamily() { if (!activeFamily.value) return; familyEditing.value = true; familyForm.value = { name: activeFamily.value.name, surname: activeFamily.value.surname, origin: activeFamily.value.origin, migrationHistory: activeFamily.value.migration_history, creed: activeFamily.value.creed, description: activeFamily.value.description }; familyVisible.value = true }
async function saveFamily() { if (!familyForm.value.name) return; try { if (familyEditing.value && activeFamily.value) { const familyID = activeFamily.value.id; await familyApi.update(familyID, familyForm.value); familyVisible.value = false; await session.loadFamilies(); session.setActiveFamily(familyID); await loadAll(); Message.success('家族资料已更新'); return } const family = await familyApi.create(familyForm.value); familyVisible.value = false; await session.loadFamilies(); session.setActiveFamily(family.id); await loadAll(); Message.success('家族已创建') } catch (error: any) { Message.error(error.message) } }
function createGenealogy() { genealogyForm.value = { name: '', description: '' }; genealogyVisible.value = true }
async function saveGenealogy() { if (!activeFamily.value || !genealogyForm.value.name) return; try { await genealogyApi.create(activeFamily.value.id, genealogyForm.value); genealogyVisible.value = false; await loadAll(); Message.success('族谱已创建') } catch (error: any) { Message.error(error.message) } }
function createPerson() { personForm.value = { name: '', gender: 'unknown', birthDate: '', birthplace: '', occupation: '', biography: '', genealogyID: genealogies.value[0]?.id || '' }; personVisible.value = true }
async function savePerson() { if (!activeFamily.value || !personForm.value.name) return; try { await personApi.create(activeFamily.value.id, personForm.value); personVisible.value = false; await loadAll(); Message.success('人物已加入档案') } catch (error: any) { Message.error(error.message) } }
async function openGraph(person: Person) { selectedPerson.value = person; selectedPersonId.value = person.id; refreshPersonFamilies(person.id); if (page.value !== 'graph') router.push({ name: 'graph' }); page.value = 'graph'; try { if (activeFamily.value) graph.value = await familyApi.graph(activeFamily.value.id, person.id) } catch (error: any) { Message.error(error.message) } }
async function changeGraphPerson(id: any) { const value = String(id || ''); if (!value) { selectedPerson.value = null; selectedPersonId.value = ''; personFamilies.value = []; if (activeFamily.value) graph.value = await familyApi.graph(activeFamily.value.id); return }; const person = persons.value.find((item) => item.id === value); if (person) await openGraph(person) }
function syncGraphPerson(id: any) { const value = String(id || ''); selectedPerson.value = value ? persons.value.find((item) => item.id === value) || null : null; selectedPersonId.value = value; refreshPersonFamilies(value) }
async function refreshPersonFamilies(id: string) { try { personFamilies.value = id ? await personApi.families(id) : [] } catch { personFamilies.value = [] } }
function openPersonFamily(p: Person) { personFamilyTarget.value = p; personFamilyForm.value = { familyId: '' }; personFamilyVisible.value = true; refreshPersonFamilyList(p.id) }
async function refreshPersonFamilyList(id: string) { try { personFamilyList.value = await personApi.families(id) } catch (e: any) { messageSafe(e) } }
async function addPersonLink() { if (!personFamilyTarget.value || !personFamilyForm.value.familyId || personFamilyBusy.value) return; personFamilyBusy.value = true; try { await personApi.linkFamily(personFamilyTarget.value.id, personFamilyForm.value.familyId); personFamilyForm.value.familyId = ''; await refreshPersonFamilyList(personFamilyTarget.value.id); await refreshPersonFamilies(personFamilyTarget.value.id); await loadAll(); Message.success('已关联家族') } catch (e: any) { Message.error(e.message) } finally { personFamilyBusy.value = false } }
async function removePersonLink(fid: string) { if (!personFamilyTarget.value || personFamilyBusy.value) return; personFamilyBusy.value = true; try { await personApi.unlinkFamily(personFamilyTarget.value.id, fid); await refreshPersonFamilyList(personFamilyTarget.value.id); await refreshPersonFamilies(personFamilyTarget.value.id); await loadAll(); Message.success('已取消关联') } catch (e: any) { Message.error(e.message) } finally { personFamilyBusy.value = false } }
async function openFamilyTree(fid: string) { if (!fid || fid === activeFamily.value?.id) return; session.setActiveFamily(fid); await loadAll() }
function messageSafe(e: any) { Message.error(e.message || '读取失败') }
async function saveRelation() { if (!selectedPerson.value || !activeFamily.value) return; try { await personApi.relation(selectedPerson.value.id, { ...relationForm.value, familyId: activeFamily.value.id }); relationVisible.value = false; graph.value = await familyApi.graph(activeFamily.value.id, selectedPerson.value.id); await loadAll(); Message.success('关系已建立') } catch (error: any) { Message.error(error.message) } }
function openRelation() { relationForm.value = { toPersonID: '', relationType: 'FATHER', customName: '', note: '' }; relationVisible.value = true }
async function reloadGraph() { if (!activeFamily.value) return; try { graph.value = await familyApi.graph(activeFamily.value.id, selectedPersonId.value) } catch (error: any) { Message.error(error.message) } }
function openMove(p: Person) { moveTarget.value = p; moveForm.value = { familyId: '' }; moveVisible.value = true }
async function saveMove() {
  if (!moveTarget.value || !moveForm.value.familyId || moveBusy.value) return
  moveBusy.value = true
  try {
    await personApi.move(moveTarget.value.id, moveForm.value.familyId)
    moveVisible.value = false
    if (selectedPerson.value?.id === moveTarget.value.id) { selectedPerson.value = null; selectedPersonId.value = '' }
    await loadAll()
    Message.success(`${moveTarget.value.name} 已移入目标家族`)
  } catch (error: any) { Message.error(error.message) } finally { moveBusy.value = false }
}
function createBook() { bookForm.value = { title: '', rootPersonID: '', contentJSON: { type: 'doc', content: [{ type: 'paragraph' }] } }; bookVisible.value = true }
async function saveBook() { if (!activeFamily.value || !bookForm.value.title) return; try { await bookApi.create(activeFamily.value.id, bookForm.value); bookVisible.value = false; await loadAll(); Message.success('家谱已创建') } catch (error: any) { Message.error(error.message) } }
async function openBook(book: Book) { try { editingBook.value = await bookApi.get(book.id); const parsed = JSON.parse(editingBook.value.content_json); bookEditorText.value = parsed.content?.map((item: any) => item.content?.map((text: any) => text.text).join('') || '').join('\n') || ''; bookEditorVisible.value = true } catch (error: any) { Message.error(error.message) } }
async function saveBookEditor() { if (!editingBook.value) return; const contentJSON = { type: 'doc', content: bookEditorText.value.split('\n').map((text) => ({ type: 'paragraph', content: text ? [{ type: 'text', text }] : undefined })) }; try { await bookApi.update(editingBook.value.id, { title: editingBook.value.title, rootPersonID: editingBook.value.root_person_id, contentJSON }); bookEditorVisible.value = false; await loadAll(); Message.success('家谱内容已保存') } catch (error: any) { Message.error(error.message) } }
async function inviteMember() { if (!activeFamily.value || !memberForm.value.email) return; try { const result = await familyApi.invite(activeFamily.value.id, memberForm.value); memberVisible.value = false; Message.success(`邀请已创建：${result.token}`) } catch (error: any) { Message.error(error.message) } }
function createPhoto() { photoForm.value = { personId: persons.value[0]?.id || '', caption: '', category: 'family', file: null }; photoVisible.value = true }
function onPhotoFileChange(event: Event) { photoForm.value.file = (event.target as HTMLInputElement).files?.[0] || null }
async function uploadPhoto() { if (!photoForm.value.file || !photoForm.value.personId) return; const data = new FormData(); data.append('file', photoForm.value.file); data.append('caption', photoForm.value.caption); data.append('category', photoForm.value.category); try { await photoApi.upload(photoForm.value.personId, data); photoVisible.value = false; await loadAll(); Message.success('照片已收录') } catch (error: any) { Message.error(error.message) } }
function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' }) }
function genderLabel(gender: string) { return gender === 'male' ? '男' : gender === 'female' ? '女' : '未填写' }
function personName(id: string) { return persons.value.find((person) => person.id === id)?.name || '未命名人物' }
</script>

<template>
  <AuthPage v-if="!session.isAuthenticated" :busy="busy" :reset-token="resetToken" :reset-requested-at="resetRequestedAt" :reset-completed-at="resetCompletedAt" @submit="submitAuth" />
  <AppLayout v-else :page="page" :menu-open="menuOpen" :nav-items="navItems" :families="session.families" :active-family="activeFamily" :user="session.user" @navigate="go" @update:menu-open="menuOpen = $event" @family-change="changeFamily" @create-family="createFamily" @logout="logout">
    <section class="content"><div class="eyebrow">{{ pageMeta[0] }}</div><h1 class="page-title">{{ pageMeta[1] }}</h1><p class="page-intro">{{ pageMeta[2] }}</p>
      <HomePage v-if="page === 'home'" :family="activeFamily" :persons-count="persons.length" :genealogies-count="genealogies.length" :books-count="books.length" :activities="activities" :format-time="formatTime" @navigate="go" />
      <GenealogyPage v-else-if="page === 'genealogy'" :genealogies="genealogies" :family-id="activeFamily?.id" :format-time="formatTime" @refresh="loadAll" />
      <PersonsPage v-else-if="page === 'persons'" :persons="persons" :query="query" :gender-label="genderLabel" :all-families="session.families" @update:query="query = $event; activeFamily && personApi.list(activeFamily.id, query).then((value) => persons = value)" @refresh="loadAll" @create="createPerson" @graph="openGraph" @move="openMove" @family="openPersonFamily" />
      <GraphPage v-else-if="page === 'graph'" :persons="persons" :graph="graph" :selected-person="selectedPerson" :selected-person-id="selectedPersonId" :person-name="personName" :person-families="personFamilies" :family-id="activeFamily?.id" @change="changeGraphPerson" @sync="syncGraphPerson" @add="openRelation" @refresh="reloadGraph" @tree="openFamilyTree" />
      <BooksPage v-else-if="page === 'books'" :books="books" :members="members" :persons="persons" :family-id="activeFamily?.id" :person-name="personName" :format-time="formatTime" @create="createBook" @open="openBook" @refresh="loadAll" />
      <PhotosPage v-else-if="page === 'photos'" :photos="photos" :persons="persons" :person-name="personName" :format-time="formatTime" @upload="createPhoto" @refresh="loadAll" />
      <MembersPage v-else-if="page === 'members'" :members="members" @invite="memberVisible = true" @refresh="loadAll" />
      <FamilyPage v-else-if="page === 'family'" :family="activeFamily" :my-role="myRole" :merge-token="mergeToken" @edit="editFamily" @merged="afterMerge" />
      <ActivityPage v-else-if="page === 'activity'" :activities="activities" :format-time="formatTime" />
    </section>
    <RecordModals :person-visible="personVisible" :relation-visible="relationVisible" :book-visible="bookVisible" :book-editor-visible="bookEditorVisible" :family-visible="familyVisible" :family-editing="familyEditing" :genealogy-visible="genealogyVisible" :member-visible="memberVisible" :photo-visible="photoVisible" :persons="persons" :genealogies="genealogies" :selected-person="selectedPerson" :editing-book="editingBook" :person-form="personForm" :relation-form="relationForm" :book-form="bookForm" :book-editor-text="bookEditorText" :family-form="familyForm" :genealogy-form="genealogyForm" :member-form="memberForm" :photo-form="photoForm" @update:person-visible="personVisible = $event" @update:relation-visible="relationVisible = $event" @update:book-visible="bookVisible = $event" @update:book-editor-visible="bookEditorVisible = $event" @update:book-editor-text="bookEditorText = $event" @update:family-visible="familyVisible = $event" @update:genealogy-visible="genealogyVisible = $event" @update:member-visible="memberVisible = $event" @update:photo-visible="photoVisible = $event" @save-person="savePerson" @save-relation="saveRelation" @save-book="saveBook" @save-book-editor="saveBookEditor" @save-family="saveFamily" @save-genealogy="saveGenealogy" @invite-member="inviteMember" @upload-photo="uploadPhoto" @photo-file-change="onPhotoFileChange" />
    <a-modal :visible="moveVisible" title="移入其他家族" :footer="false" :width="440" @update:visible="moveVisible = $event"><div class="relation-form-grid" style="display:grid;gap:12px;grid-template-columns:1fr">
      <p class="edit-hint" style="margin-top:0">将「{{ moveTarget?.name }}」连同其全部关系移入所选家族。移出后，该人物及其关系将从当前家族的关系图与人物列表中移除。</p>
      <div class="edit-field">目标家族<a-select v-model="moveForm.familyId" allow-search placeholder="选择家族"><a-option v-for="f in session.families.filter((item) => item.id !== activeFamily?.id)" :key="f.id" :value="f.id">{{ f.name }}</a-option></a-select></div>
      <p class="edit-hint">提示：若该人物与仍在本家族的人物存在关系，需要先处理这些关系（或把整个分支的人物依次移入）才能移动。</p>
      <div class="action-row"><a-button @click="moveVisible = false">取消</a-button><a-button type="primary" :loading="moveBusy" :disabled="!moveForm.familyId" @click="saveMove">移动人物</a-button></div>
    </div></a-modal>
    <a-modal :visible="personFamilyVisible" title="人物 · 家族关联" :footer="false" :width="460" @update:visible="personFamilyVisible = $event"><div class="relation-form-grid" style="display:grid;gap:12px;grid-template-columns:1fr">
      <p class="edit-hint" style="margin-top:0">「{{ personFamilyTarget?.name }}」可以被多个家族关联。每个家族的关系图只使用各自维护的关系，互不共用。</p>
      <div class="edit-field">当前关联家族<div class="fam-list"><div v-for="f in personFamilyList" :key="f.id" class="fam-item"><span>{{ f.name }}{{ f.home ? '（主家族）' : '' }}</span><a-popconfirm v-if="!f.home" content="取消该家族的关联？" @ok="removePersonLink(f.id)"><a-button type="text" status="danger" size="mini">移除</a-button></a-popconfirm></div></div></div>
      <div class="edit-field">添加关联家族<a-select v-model="personFamilyForm.familyId" allow-search placeholder="选择家族"><a-option v-for="f in session.families.filter((item) => !personFamilyList.some((pf) => pf.id === item.id))" :key="f.id" :value="f.id">{{ f.name }}</a-option></a-select></div>
      <div class="action-row"><a-button @click="personFamilyVisible = false">完成</a-button><a-button type="primary" :loading="personFamilyBusy" :disabled="!personFamilyForm.familyId" @click="addPersonLink">添加关联</a-button></div>
    </div></a-modal>
  </AppLayout>
</template>
