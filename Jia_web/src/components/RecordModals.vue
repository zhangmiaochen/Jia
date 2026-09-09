<script setup lang="ts">
import type { Book, Genealogy, Person } from '../types'

defineProps<{
  personVisible: boolean
  relationVisible: boolean
  bookVisible: boolean
  bookEditorVisible: boolean
  familyVisible: boolean
  familyEditing: boolean
  genealogyVisible: boolean
  memberVisible: boolean
  photoVisible: boolean
  persons: Person[]
  genealogies: Genealogy[]
  selectedPerson: Person | null
  editingBook: Book | null
  personForm: { name: string; gender: 'male' | 'female' | 'unknown'; birthDate: string; birthplace: string; occupation: string; biography: string; genealogyID: string }
  relationForm: { toPersonID: string; relationType: string; customName: string; note: string }
  bookForm: { title: string; rootPersonID: string }
  bookEditorText: string
  familyForm: { name: string; surname: string; origin: string; migrationHistory: string; creed: string; description: string }
  genealogyForm: { name: string; description: string }
  memberForm: { email: string; role: 'editor' | 'viewer' }
  photoForm: { personId: string; caption: string; category: string; file: File | null }
}>()

const emit = defineEmits<{
  'update:personVisible': [value: boolean]
  'update:relationVisible': [value: boolean]
  'update:bookVisible': [value: boolean]
  'update:bookEditorVisible': [value: boolean]
  'update:familyVisible': [value: boolean]
  'update:genealogyVisible': [value: boolean]
  'update:memberVisible': [value: boolean]
  'update:photoVisible': [value: boolean]
  'update:bookEditorText': [value: string]
  savePerson: []
  saveRelation: []
  saveBook: []
  saveBookEditor: []
  saveFamily: []
  saveGenealogy: []
  inviteMember: []
  uploadPhoto: []
  photoFileChange: [event: Event]
}>()
</script>

<template>
  <a-modal :visible="personVisible" title="添加人物" @update:visible="emit('update:personVisible', $event)" @ok="emit('savePerson')"><a-form :model="personForm" layout="vertical"><a-form-item field="name" label="姓名"><a-input :model-value="personForm.name" @update:model-value="personForm.name = String($event)" placeholder="填写人物姓名" /></a-form-item><a-form-item field="gender" label="性别"><a-radio-group :model-value="personForm.gender" @update:model-value="personForm.gender = $event as any"><a-radio value="male">男</a-radio><a-radio value="female">女</a-radio><a-radio value="unknown">未填写</a-radio></a-radio-group></a-form-item><a-form-item field="birthDate" label="出生日期"><a-input :model-value="personForm.birthDate" @update:model-value="personForm.birthDate = String($event)" placeholder="例如 1968-05-12" /></a-form-item><a-form-item field="birthplace" label="籍贯"><a-input :model-value="personForm.birthplace" @update:model-value="personForm.birthplace = String($event)" /></a-form-item><a-form-item field="occupation" label="职业"><a-input :model-value="personForm.occupation" @update:model-value="personForm.occupation = String($event)" /></a-form-item><a-form-item field="biography" label="人物简介"><a-textarea :model-value="personForm.biography" @update:model-value="personForm.biography = String($event)" :auto-size="{ minRows: 3 }" /></a-form-item></a-form></a-modal>
  <a-modal :visible="relationVisible" title="添加关系" @update:visible="emit('update:relationVisible', $event)" @ok="emit('saveRelation')"><a-form :model="relationForm" layout="vertical"><a-form-item field="toPersonID" label="关联人物"><a-select :model-value="relationForm.toPersonID" allow-search @update:model-value="relationForm.toPersonID = String($event)" placeholder="输入姓名搜索人物"><a-option v-for="p in persons.filter((p) => p.id !== selectedPerson?.id)" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></a-form-item><a-form-item field="relationType" label="关系"><a-select :model-value="relationForm.relationType" allow-search @update:model-value="relationForm.relationType = String($event)" ><a-option value="FATHER">父亲</a-option><a-option value="MOTHER">母亲</a-option><a-option value="SON">儿子</a-option><a-option value="DAUGHTER">女儿</a-option><a-option value="HUSBAND">丈夫</a-option><a-option value="WIFE">妻子</a-option><a-option value="BROTHER">兄弟</a-option><a-option value="SISTER">姐妹</a-option><a-option value="CUSTOM">自定义关系</a-option></a-select></a-form-item><a-form-item v-if="relationForm.relationType === 'CUSTOM'" field="customName" label="关系名称"><a-input :model-value="relationForm.customName" @update:model-value="relationForm.customName = String($event)" /></a-form-item></a-form></a-modal>
  <a-modal :visible="bookVisible" title="新建家谱" @update:visible="emit('update:bookVisible', $event)" @ok="emit('saveBook')"><a-form :model="bookForm" layout="vertical"><a-form-item field="title" label="家谱标题"><a-input :model-value="bookForm.title" @update:model-value="bookForm.title = String($event)" placeholder="例如：张三的人生记录" /></a-form-item><a-form-item field="rootPersonID" label="关联人物"><a-select :model-value="bookForm.rootPersonID" @update:model-value="bookForm.rootPersonID = String($event)" allow-clear><a-option v-for="p in persons" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></a-form-item></a-form></a-modal>
  <a-modal :visible="bookEditorVisible" :title="editingBook?.title || '编辑家谱'" width="720px" @update:visible="emit('update:bookEditorVisible', $event)" @ok="emit('saveBookEditor')"><a-form :model="editingBook || {}" layout="vertical"><a-form-item label="家谱标题"><a-input :model-value="editingBook?.title" @update:model-value="editingBook && (editingBook.title = String($event))" placeholder="例如：张氏家谱" /></a-form-item><a-form-item label="关联人物"><a-select :model-value="editingBook?.root_person_id || undefined" allow-search allow-clear placeholder="选择人物" @update:model-value="editingBook && (editingBook.root_person_id = $event ? String($event) : '')"><a-option v-for="p in persons" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></a-form-item><a-form-item label="家谱内容"><a-textarea :model-value="bookEditorText" @update:model-value="emit('update:bookEditorText', String($event))" :auto-size="{ minRows: 12, maxRows: 24 }" placeholder="写下这段家族故事……" /></a-form-item></a-form></a-modal>
  <a-modal :visible="familyVisible" :title="familyEditing ? '编辑家族资料' : '创建家族'" @update:visible="emit('update:familyVisible', $event)" @ok="emit('saveFamily')"><a-form :model="familyForm" layout="vertical"><a-form-item field="name" label="家族名称"><a-input :model-value="familyForm.name" @update:model-value="familyForm.name = String($event)" placeholder="例如：张氏家族" /></a-form-item><a-form-item field="surname" label="姓氏"><a-input :model-value="familyForm.surname" @update:model-value="familyForm.surname = String($event)" placeholder="例如：张" /></a-form-item><a-form-item field="origin" label="家族起源"><a-input :model-value="familyForm.origin" @update:model-value="familyForm.origin = String($event)" placeholder="例如：江苏苏州" /></a-form-item><a-form-item field="migrationHistory" label="迁徙历史"><a-textarea :model-value="familyForm.migrationHistory" @update:model-value="familyForm.migrationHistory = String($event)" :auto-size="{ minRows: 2 }" /></a-form-item><a-form-item field="creed" label="家训"><a-input :model-value="familyForm.creed" @update:model-value="familyForm.creed = String($event)" /></a-form-item><a-form-item field="description" label="家族简介"><a-textarea :model-value="familyForm.description" @update:model-value="familyForm.description = String($event)" :auto-size="{ minRows: 3 }" /></a-form-item></a-form></a-modal>
  <a-modal :visible="genealogyVisible" title="新建族谱" @update:visible="emit('update:genealogyVisible', $event)" @ok="emit('saveGenealogy')"><a-form :model="genealogyForm" layout="vertical"><a-form-item field="name" label="族谱名称"><a-input :model-value="genealogyForm.name" @update:model-value="genealogyForm.name = String($event)" placeholder="例如：张氏总谱" /></a-form-item><a-form-item field="description" label="说明"><a-textarea :model-value="genealogyForm.description" @update:model-value="genealogyForm.description = String($event)" :auto-size="{ minRows: 3 }" /></a-form-item></a-form></a-modal>
  <a-modal :visible="memberVisible" title="邀请成员" @update:visible="emit('update:memberVisible', $event)" @ok="emit('inviteMember')"><a-form :model="memberForm" layout="vertical"><a-form-item field="email" label="邮箱"><a-input :model-value="memberForm.email" @update:model-value="memberForm.email = String($event)" placeholder="name@example.com" /></a-form-item><a-form-item field="role" label="成员角色"><a-select :model-value="memberForm.role" @update:model-value="memberForm.role = String($event) as any" ><a-option value="editor">编辑者</a-option><a-option value="viewer">只读成员</a-option></a-select></a-form-item></a-form></a-modal>
  <a-modal :visible="photoVisible" title="上传照片" @update:visible="emit('update:photoVisible', $event)" @ok="emit('uploadPhoto')"><a-form :model="photoForm" layout="vertical"><a-form-item field="personId" label="关联人物"><a-select :model-value="photoForm.personId" @update:model-value="photoForm.personId = String($event)" placeholder="选择人物"><a-option v-for="p in persons" :key="p.id" :value="p.id">{{ p.name }}</a-option></a-select></a-form-item><a-form-item field="caption" label="照片说明"><a-input :model-value="photoForm.caption" @update:model-value="photoForm.caption = String($event)" /></a-form-item><a-form-item field="category" label="分类"><a-select :model-value="photoForm.category" @update:model-value="photoForm.category = String($event)" ><a-option value="portrait">人物肖像</a-option><a-option value="family">家庭照片</a-option><a-option value="event">重要事件</a-option><a-option value="other">其他</a-option></a-select></a-form-item><a-form-item label="照片文件"><input type="file" accept="image/jpeg,image/png,image/webp" @change="emit('photoFileChange', $event)"></a-form-item></a-form></a-modal>
</template>
