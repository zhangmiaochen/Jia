import { api } from './client'
import type { Activity, Book, Family, FamilyMember, Genealogy, GraphData, Person, PersonFamily, Photo, Relation, User } from '../types'

export const authApi = {
  register: (payload: { email: string; password: string; displayName: string }) => api.post<{ token: string; user: User }>('/auth/register', payload),
  login: (payload: { email: string; password: string }) => api.post<{ token: string; user: User }>('/auth/login', payload),
  forgotPassword: (email: string) => api.post<{ message: string; reset_token: string; expires_at: string }>('/auth/forgot-password', { email }),
  resetPassword: (token: string, password: string) => api.post<{ reset: boolean }>('/auth/reset-password', { token, password }),
  me: () => api.get<User>('/auth/me'),
}
export const familyApi = {
  list: () => api.get<Family[]>('/families'),
  create: (payload: Partial<Family>) => api.post<Family>('/families', payload),
  get: (id: string) => api.get<Family>(`/families/${id}`),
  update: (id: string, payload: Partial<Family>) => api.patch<Family>(`/families/${id}`, payload),
  members: (id: string) => api.get<FamilyMember[]>(`/families/${id}/members`),
  addMember: (id: string, payload: { email: string; role: 'editor' | 'viewer' }) => api.post<{ added: boolean }>(`/families/${id}/members`, payload),
  updateMember: (id: string, payload: { role: 'owner' | 'editor' | 'viewer' }) => api.patch<FamilyMember>(`/members/${id}`, payload),
  removeMember: (id: string) => api.delete<{ deleted: boolean }>(`/members/${id}`),
  invite: (id: string, payload: { email: string; role: 'editor' | 'viewer' }) => api.post<{ token: string; expires_at: string }>(`/families/${id}/invites`, payload),
  acceptInvite: (token: string) => api.post<{ family_id: string; joined: boolean }>(`/invites/${token}/accept`),
  activities: (id: string) => api.get<Activity[]>(`/families/${id}/activities`),
  graph: (id: string, centerPersonID = '') => api.get<GraphData>(`/families/${id}/graph`, { params: centerPersonID ? { center_person_id: centerPersonID } : undefined }),
}
export const genealogyApi = {
  list: (familyId: string) => api.get<Genealogy[]>(`/families/${familyId}/genealogies`),
  create: (familyId: string, payload: { name: string; description?: string }) => api.post<Genealogy>(`/families/${familyId}/genealogies`, payload),
  get: (id: string) => api.get<Genealogy>(`/genealogies/${id}`),
  update: (id: string, payload: { name: string; description?: string }) => api.patch<Genealogy>(`/genealogies/${id}`, payload),
  remove: (id: string) => api.delete<{ deleted: boolean }>(`/genealogies/${id}`),
  persons: (id: string) => api.get<Person[]>(`/genealogies/${id}/persons`),
  addPerson: (id: string, personID: string) => api.post<{ genealogy_id: string; person_id: string }>(`/genealogies/${id}/persons`, { person_id: personID }),
  removePerson: (id: string, personID: string) => api.delete<{ deleted: boolean }>(`/genealogies/${id}/persons/${personID}`),
}
export const personApi = {
  list: (familyId: string, q = '') => api.get<Person[]>(`/families/${familyId}/persons`, { params: { q } }),
  create: (familyId: string, payload: Partial<Person> & { genealogyID?: string }) => api.post<Person>(`/families/${familyId}/persons`, payload),
  update: (id: string, payload: { name: string; gender: string; birthDate: string; deathDate: string; birthplace: string; occupation: string; biography: string }) => api.patch<Person>(`/persons/${id}`, payload),
  get: (id: string) => api.get<Person>(`/persons/${id}`),
  remove: (id: string) => api.delete<{ deleted: boolean }>(`/persons/${id}`),
  genealogies: (id: string) => api.get<Genealogy[]>(`/persons/${id}/genealogies`),
  books: (id: string) => api.get<Book[]>(`/persons/${id}/books`),
  photos: (id: string) => api.get<Photo[]>(`/persons/${id}/photos`),
  graph: (id: string) => api.get<GraphData>(`/persons/${id}/graph`),
  claim: (id: string) => api.post(`/persons/${id}/claim`),
  families: (id: string) => api.get<PersonFamily[]>(`/persons/${id}/families`),
  linkFamily: (id: string, familyId: string) => api.post<{ linked: boolean }>(`/persons/${id}/families`, { family_id: familyId }),
  unlinkFamily: (id: string, familyId: string) => api.delete<{ removed: boolean }>(`/persons/${id}/families/${familyId}`),
  move: (id: string, familyId: string) => api.post<{ moved: boolean }>(`/persons/${id}/move`, { family_id: familyId }),
  relation: (id: string, payload: { toPersonID: string; relationType: string; customName?: string; note?: string; familyId?: string }) => api.post(`/persons/${id}/relations`, payload),
  updateRelation: (id: string, payload: { from_person_id: string; to_person_id: string; relation_type: string; custom_name?: string; note?: string }) => api.patch<Relation>(`/relations/${id}`, payload),
  removeRelation: (id: string) => api.delete<{ deleted: boolean }>(`/relations/${id}`),
}
export const bookApi = {
  list: (familyId: string) => api.get<Book[]>(`/families/${familyId}/books`),
  create: (familyId: string, payload: { title: string; rootPersonID?: string; contentJSON?: object }) => api.post<Book>(`/families/${familyId}/books`, payload),
  get: (id: string) => api.get<Book>(`/books/${id}`),
  update: (id: string, payload: { title: string; rootPersonID?: string; contentJSON?: object }) => api.patch<Book>(`/books/${id}`, payload),
  remove: (id: string) => api.delete<{ deleted: boolean }>(`/books/${id}`),
  collaborators: (id: string) => api.get<User[]>(`/books/${id}/collaborators`),
  addCollaborator: (id: string, userID: string) => api.post<{ book_id: string; user_id: string }>(`/books/${id}/collaborators`, { user_id: userID }),
  removeCollaborator: (id: string, userID: string) => api.delete<{ deleted: boolean }>(`/books/${id}/collaborators/${userID}`),
}
export const photoApi = {
  list: (familyId: string) => api.get<Photo[]>(`/families/${familyId}/photos`),
  get: (id: string) => api.get<Photo>(`/photos/${id}`),
  update: (id: string, payload: Partial<Pick<Photo, 'caption' | 'category' | 'taken_at' | 'location'>>) => api.patch<Photo>(`/photos/${id}`, payload),
  upload: (personId: string, data: FormData) => api.post<Photo>(`/persons/${personId}/photos`, data, { headers: { 'Content-Type': 'multipart/form-data' } }),
  remove: (id: string) => api.delete<{ deleted: boolean }>(`/photos/${id}`),
  persons: (id: string) => api.get<Person[]>(`/photos/${id}/persons`),
  addPerson: (id: string, personID: string) => api.post<{ photo_id: string; person_id: string }>(`/photos/${id}/persons`, { person_id: personID }),
  removePerson: (id: string, personID: string) => api.delete<{ deleted: boolean }>(`/photos/${id}/persons/${personID}`),
}
