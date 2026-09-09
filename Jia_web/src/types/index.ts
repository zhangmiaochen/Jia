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
