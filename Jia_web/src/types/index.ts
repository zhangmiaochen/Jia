export interface User { id: string; email: string; display_name: string; avatar_url: string; created_at: string }
export interface Family { id: string; name: string; surname: string; origin: string; migration_history: string; creed: string; description: string; created_by: string; created_at: string; updated_at: string; person_count?: number; genealogy_count?: number; member_count?: number }
export interface Genealogy { id: string; family_id: string; name: string; description: string; created_by: string; created_at: string; updated_at: string; person_count?: number }
export interface Person { id: string; family_id: string; name: string; gender: 'male' | 'female' | 'unknown'; birth_date: string; death_date: string; birthplace: string; occupation: string; biography: string; claimed_by: string; created_at: string; updated_at: string }
export interface Book { id: string; family_id: string; title: string; root_person_id: string; content_json: string; created_by: string; created_at: string; updated_at: string }
export interface Relation { id: string; from_person_id: string; to_person_id: string; relation_type: string; custom_name: string; note: string }
export interface GraphData { center: string; nodes: Person[]; edges: Relation[] }
export interface Activity { id: string; actor_id: string; type: string; target_type: string; target_id: string; summary: string; created_at: string }
export interface Photo { id: string; family_id: string; person_id: string; path: string; original_name: string; caption: string; category: string; taken_at: string; location: string; created_at: string }
export interface FamilyMember { id: string; user_id: string; email: string; display_name: string; avatar_url: string; role: 'owner' | 'editor' | 'viewer'; status: string; joined_at: string }
export interface PersonFamily { id: string; name: string; surname: string; home: boolean }
export interface FamilyBrief { id: string; name: string; surname: string; origin: string; created_at?: string; person_count: number; genealogy_count: number; member_count: number; book_count: number; photo_count: number; relation_count: number; activity_count: number }
export interface PersonBrief { id: string; family_id: string; name: string; gender: 'male' | 'female' | 'unknown'; birth_date: string; death_date: string; birthplace: string; occupation: string; claimed_by: string; relation_count: number; genealogy_count: number; photo_count: number }
export interface MergeSuggestion { source_person_id: string; target_person_id: string; reason: string; confidence: 'high' | 'medium' | 'low' }
export interface GenealogyPair { name: string; source_id: string; target_id: string }
export interface MergeInvite { token: string; expires_at: string; target_family: FamilyBrief | null; my_families: FamilyBrief[]; created_at?: string }
export interface MergePreview { token: string; expires_at: string; source_family: FamilyBrief; target_family: FamilyBrief; source_persons: PersonBrief[]; target_persons: PersonBrief[]; suggestions: MergeSuggestion[]; same_name_genealogies: GenealogyPair[]; my_role_in_source: string; warnings: string[] }
export interface MergeSummary { merge_id: string; person_count: number; persons_moved: number; persons_merged: number; relations_moved: number; genealogies_moved: number; genealogies_merged: number; books_moved: number; photos_moved: number; activities_moved: number; members_joined: number; members_upgraded: number; source_family?: FamilyBrief; target_family?: FamilyBrief; warnings: string[] }
