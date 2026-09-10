package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// 家族合并：同一家族被几个人分别录入成多份档案后，把 source 家族整体并入 target 家族。
//
// 流程（双方 owner 共同确认）：
//  1. 主家族（target）owner 调用 createMergeInvite 生成合并码；
//  2. 被并入方（source）owner 把合并码交给 previewMerge，拿到人物对照建议与搬迁清单；
//  3. 被并入方 owner 确认人物对照后调用 executeMerge，服务端在一个事务里完成搬迁。
//
// 说明：合并完成后 source 家族的内容全部迁入 target，source 的 members 被清空（因此不再出现在家族切换器里），
// 家族行本身保留为可追溯的空壳；family_merges / family_merge_pairs 记录合并码与人物对照结果。

// 与 migrations/001_init.sql 中同名定义保持一致：老库无需先跑 migrate，重启服务即可使用合并功能。
const mergeSchema = `
CREATE TABLE IF NOT EXISTS family_merges (
  id TEXT PRIMARY KEY,
  token TEXT NOT NULL UNIQUE,
  target_family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  source_family_id TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  redeemed_by TEXT NOT NULL DEFAULT '',
  executed_at TEXT NOT NULL DEFAULT '',
  summary_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS family_merge_pairs (
  merge_id TEXT NOT NULL REFERENCES family_merges(id) ON DELETE CASCADE,
  source_person_id TEXT NOT NULL,
  source_person_name TEXT NOT NULL DEFAULT '',
  target_person_id TEXT NOT NULL,
  PRIMARY KEY (merge_id, source_person_id)
);
CREATE INDEX IF NOT EXISTS idx_family_merges_target ON family_merges(target_family_id);
CREATE INDEX IF NOT EXISTS idx_family_merges_source ON family_merges(source_family_id);
`

func ensureMergeSchema(db *sql.DB) error {
	_, err := db.Exec(mergeSchema)
	return err
}

// ---------- 合并码 ----------

// 去掉容易看错的 0/O/1/I，避免家人抄错码。
const mergeCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newMergeCode() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	parts := make([]string, 0, 3)
	for i := 0; i < 12; i += 4 {
		part := make([]byte, 4)
		for j := 0; j < 4; j++ {
			part[j] = mergeCodeAlphabet[int(b[i+j])%len(mergeCodeAlphabet)]
		}
		parts = append(parts, string(part))
	}
	return strings.Join(parts, "-")
}

// 容忍大小写、空格与漏输/多输的连字符。
func normalizeMergeCode(v string) string {
	raw := strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\t", "").Replace(strings.TrimSpace(v)))
	if len(raw) == 12 {
		return raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12]
	}
	return strings.ToUpper(strings.TrimSpace(v))
}

type mergeInvite struct {
	ID             string
	Token          string
	TargetFamilyID string
	SourceFamilyID string
	CreatedBy      string
	CreatedAt      string
	ExpiresAt      string
	RedeemedBy     string
	ExecutedAt     string
}

func (a *app) loadMergeInvite(token string) (mergeInvite, error) {
	var m mergeInvite
	err := a.db.QueryRow(`SELECT id,token,target_family_id,COALESCE(source_family_id,''),created_by,created_at,expires_at,
		COALESCE(redeemed_by,''),COALESCE(executed_at,'') FROM family_merges WHERE token=?`, token).
		Scan(&m.ID, &m.Token, &m.TargetFamilyID, &m.SourceFamilyID, &m.CreatedBy, &m.CreatedAt, &m.ExpiresAt, &m.RedeemedBy, &m.ExecutedAt)
	return m, err
}

// checkMergeInvite 校验合并码可用性；返回 false 表示已经写过错误响应。
func (a *app) checkMergeInvite(w http.ResponseWriter, token string) (mergeInvite, bool) {
	m, err := a.loadMergeInvite(token)
	if err != nil {
		errorJSON(w, http.StatusNotFound, "merge_code_invalid", "合并码无效，请向对方确认")
		return m, false
	}
	if m.ExecutedAt != "" {
		errorJSON(w, http.StatusConflict, "merge_code_used", "该合并码已经使用过了")
		return m, false
	}
	if t, err := time.Parse(time.RFC3339, m.ExpiresAt); err == nil && time.Now().After(t) {
		errorJSON(w, http.StatusGone, "merge_code_expired", "合并码已过期（有效期 7 天），请让对方重新生成")
		return m, false
	}
	return m, true
}

// ---------- 家族信息 ----------

func (a *app) familyBrief(fid string) map[string]any {
	var id, name, surname, origin, createdAt string
	if a.db.QueryRow("SELECT id,name,surname,origin,created_at FROM families WHERE id=?", fid).Scan(&id, &name, &surname, &origin, &createdAt) != nil {
		return nil
	}
	count := func(q string) int {
		var n int
		_ = a.db.QueryRow(q, fid).Scan(&n)
		return n
	}
	return map[string]any{
		"id": id, "name": name, "surname": surname, "origin": origin, "created_at": createdAt,
		"person_count":    count("SELECT COUNT(*) FROM persons WHERE family_id=?"),
		"genealogy_count": count("SELECT COUNT(*) FROM genealogies WHERE family_id=?"),
		"member_count":    count("SELECT COUNT(*) FROM family_members WHERE family_id=?"),
		"book_count":      count("SELECT COUNT(*) FROM books WHERE family_id=?"),
		"photo_count":     count("SELECT COUNT(*) FROM photos WHERE family_id=?"),
		"relation_count":  count("SELECT COUNT(*) FROM person_relations WHERE family_id=?"),
		"activity_count":  count("SELECT COUNT(*) FROM activities WHERE family_id=?"),
	}
}

// 当前用户拥有 owner 权限的家族（用于选择「把哪个家族并入」）。
func (a *app) ownerFamilies(uid, exclude string) []map[string]any {
	rows, err := a.db.Query(`SELECT f.id,f.name,f.surname,
		(SELECT COUNT(*) FROM persons p WHERE p.family_id=f.id)
		FROM families f JOIN family_members m ON m.family_id=f.id
		WHERE m.user_id=? AND m.role='owner' AND m.status='active' AND f.id<>?
		ORDER BY f.updated_at DESC`, uid, exclude)
	out := []map[string]any{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, surname string
		var persons int
		if rows.Scan(&id, &name, &surname, &persons) == nil {
			out = append(out, map[string]any{"id": id, "name": name, "surname": surname, "person_count": persons})
		}
	}
	return out
}

// ---------- 接口：生成合并码 ----------

func (a *app) createMergeInvite(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	now := time.Now().UTC()
	token := newMergeCode()
	for i := 0; i < 6; i++ {
		var exists int
		if a.db.QueryRow("SELECT COUNT(*) FROM family_merges WHERE token=?", token).Scan(&exists) == nil && exists == 0 {
			break
		}
		token = newMergeCode()
	}
	expires := now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := a.db.Exec("INSERT INTO family_merges(id,token,target_family_id,created_by,created_at,expires_at) VALUES(?,?,?,?,?,?)",
		newID(), token, fid, userID(r.Context()), now.Format(time.RFC3339), expires); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "生成合并码失败")
		return
	}
	a.activity(fid, userID(r.Context()), "family.merge_code_created", "family", fid, "生成了家族合并码")
	jsonResponse(w, http.StatusCreated, map[string]any{
		"token": token, "expires_at": expires, "target_family": a.familyBrief(fid),
	})
}

// ---------- 接口：查询合并码指向的家族 ----------

func (a *app) getMergeInvite(w http.ResponseWriter, r *http.Request) {
	token := normalizeMergeCode(chi.URLParam(r, "token"))
	m, ok := a.checkMergeInvite(w, token)
	if !ok {
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"token":          m.Token,
		"expires_at":     m.ExpiresAt,
		"target_family":  a.familyBrief(m.TargetFamilyID),
		"my_families":    a.ownerFamilies(userID(r.Context()), m.TargetFamilyID),
		"created_at":     m.CreatedAt,
		"already_merged": m.SourceFamilyID,
	})
}

// ---------- 人物对照建议 ----------

type personBrief struct {
	ID             string `json:"id"`
	FamilyID       string `json:"family_id"`
	Name           string `json:"name"`
	Gender         string `json:"gender"`
	BirthDate      string `json:"birth_date"`
	DeathDate      string `json:"death_date"`
	Birthplace     string `json:"birthplace"`
	Occupation     string `json:"occupation"`
	ClaimedBy      string `json:"claimed_by"`
	RelationCount  int    `json:"relation_count"`
	GenealogyCount int    `json:"genealogy_count"`
	PhotoCount     int    `json:"photo_count"`
}

func (a *app) familyPersonsBrief(fid string) []personBrief {
	rows, err := a.db.Query(`SELECT p.id,p.family_id,p.name,p.gender,p.birth_date,p.death_date,p.birthplace,p.occupation,COALESCE(p.claimed_by,''),
		(SELECT COUNT(*) FROM person_relations r WHERE (r.from_person_id=p.id OR r.to_person_id=p.id)),
		(SELECT COUNT(*) FROM genealogy_persons gp WHERE gp.person_id=p.id),
		(SELECT COUNT(*) FROM photos ph WHERE ph.person_id=p.id)
		FROM persons p WHERE p.family_id=? ORDER BY p.name,p.created_at`, fid)
	out := []personBrief{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var p personBrief
		if rows.Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.ClaimedBy, &p.RelationCount, &p.GenealogyCount, &p.PhotoCount) == nil {
			out = append(out, p)
		}
	}
	return out
}

type mergeSuggestion struct {
	SourcePersonID string `json:"source_person_id"`
	TargetPersonID string `json:"target_person_id"`
	Reason         string `json:"reason"`
	Confidence     string `json:"confidence"`
}

// 建议配对规则：姓名完全相同，且出生日期一致（或至少一方未填）。同名候选多于一个且无法用出生日期区分时不给建议。
func mergeSuggestions(source, target []personBrief) []mergeSuggestion {
	byName := map[string][]personBrief{}
	for _, t := range target {
		key := strings.TrimSpace(t.Name)
		byName[key] = append(byName[key], t)
	}
	used := map[string]bool{}
	out := []mergeSuggestion{}
	for _, s := range source {
		candidates := byName[strings.TrimSpace(s.Name)]
		if len(candidates) == 0 {
			continue
		}
		pick := ""
		reason := ""
		confidence := ""
		birth := strings.TrimSpace(s.BirthDate)
		if birth != "" {
			for _, c := range candidates {
				if used[c.ID] {
					continue
				}
				if strings.TrimSpace(c.BirthDate) == birth {
					pick, reason, confidence = c.ID, "姓名与出生日期一致", "high"
					break
				}
			}
		}
		if pick == "" {
			free := []personBrief{}
			for _, c := range candidates {
				if !used[c.ID] {
					free = append(free, c)
				}
			}
			if len(free) == 1 && (birth == "" || strings.TrimSpace(free[0].BirthDate) == "") {
				pick, reason, confidence = free[0].ID, "姓名一致，出生日期缺失", "medium"
			} else if len(free) == 1 && len(candidates) == 1 {
				pick, reason, confidence = free[0].ID, "姓名一致（出生日期不同，请确认）", "low"
			}
		}
		if pick == "" {
			continue
		}
		used[pick] = true
		out = append(out, mergeSuggestion{SourcePersonID: s.ID, TargetPersonID: pick, Reason: reason, Confidence: confidence})
	}
	return out
}

type genealogyPair struct {
	Name       string `json:"name"`
	SourceID   string `json:"source_id"`
	TargetID   string `json:"target_id"`
	SourceName string `json:"source_genealogy_name"`
	TargetName string `json:"target_genealogy_name"`
}

// 两边同名的族谱：可以选择合并成一份。
func (a *app) sameNameGenealogies(source, target string) []genealogyPair {
	load := func(fid string) map[string][]string {
		out := map[string][]string{}
		rows, err := a.db.Query("SELECT id,name FROM genealogies WHERE family_id=?", fid)
		if err != nil {
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var id, name string
			if rows.Scan(&id, &name) == nil {
				key := strings.TrimSpace(name)
				out[key] = append(out[key], id)
			}
		}
		return out
	}
	src, tgt := load(source), load(target)
	out := []genealogyPair{}
	for name, sids := range src {
		tids, ok := tgt[name]
		if !ok || name == "" || len(sids) != 1 || len(tids) != 1 {
			continue
		}
		out = append(out, genealogyPair{Name: name, SourceID: sids[0], TargetID: tids[0], SourceName: name, TargetName: name})
	}
	return out
}

// ---------- 接口：合并预览 ----------

func (a *app) previewMerge(w http.ResponseWriter, r *http.Request) {
	token := normalizeMergeCode(chi.URLParam(r, "token"))
	m, ok := a.checkMergeInvite(w, token)
	if !ok {
		return
	}
	var in struct {
		SourceFamilyID string `json:"source_family_id"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.SourceFamilyID) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择要并入的家族")
		return
	}
	source, target := in.SourceFamilyID, m.TargetFamilyID
	if source == target {
		errorJSON(w, http.StatusBadRequest, "same_family", "不能把家族并入它自己")
		return
	}
	if a.familyBrief(source) == nil || a.familyBrief(target) == nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家族不存在")
		return
	}
	if a.memberRole(source, userID(r.Context())) != "owner" {
		errorJSON(w, http.StatusForbidden, "forbidden", "只有该家族的所有者（owner）才能把它并入其他家族")
		return
	}
	srcPersons := a.familyPersonsBrief(source)
	tgtPersons := a.familyPersonsBrief(target)
	suggestions := mergeSuggestions(srcPersons, tgtPersons)
	warnings := []string{}
	// 认领冲突提示
	claimed := map[string]string{}
	for _, t := range tgtPersons {
		if t.ClaimedBy != "" {
			claimed[t.ID] = t.ClaimedBy
		}
	}
	for _, s := range suggestions {
		if id, ok := claimed[s.TargetPersonID]; ok {
			for _, sp := range srcPersons {
				if sp.ID == s.SourcePersonID && sp.ClaimedBy != "" && sp.ClaimedBy != id {
					warnings = append(warnings, fmt.Sprintf("「%s」在两边分别被不同账号认领，合并后保留接收方档案的认领关系", sp.Name))
				}
			}
		}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"token":                 m.Token,
		"expires_at":            m.ExpiresAt,
		"source_family":         a.familyBrief(source),
		"target_family":         a.familyBrief(target),
		"source_persons":        srcPersons,
		"target_persons":        tgtPersons,
		"suggestions":           suggestions,
		"same_name_genealogies": a.sameNameGenealogies(source, target),
		"my_role_in_source":     a.memberRole(source, userID(r.Context())),
		"warnings":              warnings,
	})
}

// ---------- 接口：执行合并 ----------

type mergePair struct {
	SourcePersonID string `json:"source_person_id"`
	TargetPersonID string `json:"target_person_id"`
}

func (a *app) executeMerge(w http.ResponseWriter, r *http.Request) {
	token := normalizeMergeCode(chi.URLParam(r, "token"))
	m, ok := a.checkMergeInvite(w, token)
	if !ok {
		return
	}
	var in struct {
		SourceFamilyID           string      `json:"source_family_id"`
		Pairs                    []mergePair `json:"pairs"`
		MergeSameNameGenealogies *bool       `json:"merge_same_name_genealogies"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.SourceFamilyID) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择要并入的家族")
		return
	}
	source, target := in.SourceFamilyID, m.TargetFamilyID
	if source == target {
		errorJSON(w, http.StatusBadRequest, "same_family", "不能把家族并入它自己")
		return
	}
	actor := userID(r.Context())
	if a.memberRole(source, actor) != "owner" {
		errorJSON(w, http.StatusForbidden, "forbidden", "只有该家族的所有者（owner）才能确认合并")
		return
	}
	if a.familyBrief(source) == nil || a.familyBrief(target) == nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家族不存在")
		return
	}
	// 校验人物对照：source 必须是本家族人物、target 必须是目标家族人物，且一一对应
	seenSource := map[string]bool{}
	seenTarget := map[string]bool{}
	pairs := []mergePair{}
	for _, p := range in.Pairs {
		if p.SourcePersonID == "" || p.TargetPersonID == "" {
			continue
		}
		if p.SourcePersonID == p.TargetPersonID {
			errorJSON(w, http.StatusBadRequest, "invalid_pair", "人物对照无效")
			return
		}
		if seenSource[p.SourcePersonID] || seenTarget[p.TargetPersonID] {
			errorJSON(w, http.StatusBadRequest, "invalid_pair", "同一个人物不能被对照两次")
			return
		}
		var srcFam, tgtFam string
		if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", p.SourcePersonID).Scan(&srcFam) != nil || srcFam != source {
			errorJSON(w, http.StatusBadRequest, "invalid_pair", "被并入的人物不属于本家族")
			return
		}
		if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", p.TargetPersonID).Scan(&tgtFam) != nil || tgtFam != target {
			errorJSON(w, http.StatusBadRequest, "invalid_pair", "合并目标人物不属于接收家族")
			return
		}
		seenSource[p.SourcePersonID] = true
		seenTarget[p.TargetPersonID] = true
		pairs = append(pairs, p)
	}
	mergeSameName := true
	if in.MergeSameNameGenealogies != nil {
		mergeSameName = *in.MergeSameNameGenealogies
	}

	// 合并前先留存两边的家族信息（合并后 source 已被搬空，快照更便于事后追溯）
	sourceBrief, targetBrief := a.familyBrief(source), a.familyBrief(target)
	summary, warnings, err := a.mergeFamilies(m.ID, source, target, actor, pairs, mergeSameName)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "merge_failed", "合并失败，数据未改动："+err.Error())
		return
	}
	summary["source_family"] = sourceBrief
	summary["target_family"] = targetBrief
	summary["warnings"] = warnings
	jsonResponse(w, http.StatusOK, summary)
}

// mergeFamilies 在一个事务里把 source 家族整体并入 target 家族。
func (a *app) mergeFamilies(mergeID, source, target, actor string, pairs []mergePair, mergeSameName bool) (map[string]any, []string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	warnings := []string{}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, warnings, err
	}
	defer tx.Rollback()

	exec := func(query string, args ...any) (int64, error) {
		res, err := tx.Exec(query, args...)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		return n, nil
	}
	touchedFamilies := map[string]bool{source: true, target: true}

	// 1) 人物：先按对照表合并重复档案，其余人物随后整体搬迁
	personsMerged := 0
	relationsDropped := 0
	for _, p := range pairs {
		fams, w, dropped, err := mergePersonInto(tx, p.SourcePersonID, p.TargetPersonID, now)
		if err != nil {
			return nil, warnings, err
		}
		relationsDropped += dropped
		for _, f := range fams {
			touchedFamilies[f] = true
		}
		warnings = append(warnings, w...)
		if w := mergePairRecord(tx, mergeID, p); w != nil {
			return nil, warnings, w
		}
		personsMerged++
	}

	// 2) 同名族谱合并（必须在 family_id 搬迁之前，用各自的家族标识匹配）
	genealogiesMerged := 0
	if mergeSameName {
		for _, g := range a.sameNameGenealogies(source, target) {
			if _, err := exec("INSERT OR IGNORE INTO genealogy_persons(genealogy_id,person_id) SELECT ?,person_id FROM genealogy_persons WHERE genealogy_id=?", g.TargetID, g.SourceID); err != nil {
				return nil, warnings, err
			}
			if _, err := exec("DELETE FROM genealogy_persons WHERE genealogy_id=?", g.SourceID); err != nil {
				return nil, warnings, err
			}
			if _, err := exec("DELETE FROM genealogies WHERE id=?", g.SourceID); err != nil {
				return nil, warnings, err
			}
			genealogiesMerged++
		}
	}

	// 3) 关系：先清掉与目标家族已存在的同型关系，避免唯一约束冲突
	relationsDeduped, err := exec(`DELETE FROM person_relations WHERE family_id=? AND EXISTS (
		SELECT 1 FROM person_relations t WHERE t.family_id=?
		  AND t.from_person_id=person_relations.from_person_id
		  AND t.to_person_id=person_relations.to_person_id
		  AND t.relation_type=person_relations.relation_type
		  AND t.custom_name=person_relations.custom_name)`, source, target)
	if err != nil {
		return nil, warnings, err
	}
	relationsMoved, err := exec("UPDATE person_relations SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}
	if relationsDeduped > 0 {
		warnings = append(warnings, fmt.Sprintf("两边有 %d 条完全相同的关系，已自动去重保留一份", relationsDeduped))
	}

	// 4) 人物-家族关联：source 的关联记录转到 target（主家族随后也会变成 target）
	if _, err := exec("INSERT OR IGNORE INTO person_families(person_id,family_id) SELECT person_id,? FROM person_families WHERE family_id=?", target, source); err != nil {
		return nil, warnings, err
	}
	if _, err := exec("DELETE FROM person_families WHERE family_id=?", source); err != nil {
		return nil, warnings, err
	}

	// 5) 内容整体搬迁
	personsMoved, err := exec("UPDATE persons SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}
	genealogiesMoved, err := exec("UPDATE genealogies SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}
	booksMoved, err := exec("UPDATE books SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}
	photosMoved, err := exec("UPDATE photos SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}
	activitiesMoved, err := exec("UPDATE activities SET family_id=? WHERE family_id=?", target, source)
	if err != nil {
		return nil, warnings, err
	}

	// 6) 成员并入：同角色保留；被并入方 owner 在原家族是 owner，这里同样保留 owner 权限
	membersJoined, membersUpgraded := 0, 0
	type memberRow struct{ userID, role, status, joinedAt string }
	members := []memberRow{}
	rows, err := tx.Query("SELECT user_id,role,status,joined_at FROM family_members WHERE family_id=?", source)
	if err != nil {
		return nil, warnings, err
	}
	for rows.Next() {
		var mr memberRow
		if rows.Scan(&mr.userID, &mr.role, &mr.status, &mr.joinedAt) == nil {
			members = append(members, mr)
		}
	}
	rows.Close()
	for _, mr := range members {
		var existing string
		found := tx.QueryRow("SELECT role FROM family_members WHERE family_id=? AND user_id=?", target, mr.userID).Scan(&existing) == nil
		if !found {
			if _, err := exec("INSERT INTO family_members(id,family_id,user_id,role,status,joined_at) VALUES(?,?,?,?,?,?)", newID(), target, mr.userID, mr.role, mr.status, mr.joinedAt); err != nil {
				return nil, warnings, err
			}
			membersJoined++
			continue
		}
		if mr.role == "owner" && existing != "owner" {
			if _, err := exec("UPDATE family_members SET role='owner' WHERE family_id=? AND user_id=?", target, mr.userID); err != nil {
				return nil, warnings, err
			}
			membersUpgraded++
		}
	}

	// 7) 清掉被并入家族的待接受邀请与成员（成员清空后该家族不再出现在任何人的家族列表里）
	if _, err := exec("DELETE FROM invites WHERE family_id=? AND accepted_at=''", source); err != nil {
		return nil, warnings, err
	}
	if _, err := exec("DELETE FROM family_members WHERE family_id=?", source); err != nil {
		return nil, warnings, err
	}

	// 8) 修复受影响家族的关系正反指向
	list := make([]string, 0, len(touchedFamilies))
	for f := range touchedFamilies {
		list = append(list, f)
	}
	if err := repairInverseLinks(tx, list); err != nil {
		return nil, warnings, err
	}

	summary := map[string]any{
		"merge_id":           mergeID,
		"person_count":       personsMoved + int64(personsMerged),
		"persons_moved":      personsMoved,
		"persons_merged":     personsMerged,
		"relations_moved":    relationsMoved,
		"relations_deduped":  relationsDeduped + int64(relationsDropped),
		"genealogies_moved":  genealogiesMoved,
		"genealogies_merged": genealogiesMerged,
		"books_moved":        booksMoved,
		"photos_moved":       photosMoved,
		"activities_moved":   activitiesMoved,
		"members_joined":     membersJoined,
		"members_upgraded":   membersUpgraded,
	}
	encoded, _ := json.Marshal(summary)
	if _, err := exec("UPDATE family_merges SET source_family_id=?,redeemed_by=?,executed_at=?,summary_json=? WHERE id=?",
		source, actor, now, string(encoded), mergeID); err != nil {
		return nil, warnings, err
	}
	srcName := ""
	var sName string
	if tx.QueryRow("SELECT name FROM families WHERE id=?", source).Scan(&sName) == nil {
		srcName = sName
	}
	text := fmt.Sprintf("家族合并：已把「%s」的 %d 位人物、%d 份族谱、%d 部家谱并入本家族（其中 %d 位重复人物已按对照合并）",
		srcName, personsMoved+int64(personsMerged), genealogiesMoved, booksMoved, personsMerged)
	if _, err := exec("INSERT INTO activities(id,family_id,actor_id,type,target_type,target_id,summary,created_at) VALUES(?,?,?,?,?,?,?,?)",
		newID(), target, actor, "family.merged", "family", source, text, now); err != nil {
		return nil, warnings, err
	}
	if err := tx.Commit(); err != nil {
		return nil, warnings, err
	}
	return summary, warnings, nil
}

func mergePairRecord(tx *sql.Tx, mergeID string, p mergePair) error {
	var name string
	_ = tx.QueryRow("SELECT name FROM persons WHERE id=?", p.SourcePersonID).Scan(&name)
	_, err := tx.Exec("INSERT OR REPLACE INTO family_merge_pairs(merge_id,source_person_id,source_person_name,target_person_id) VALUES(?,?,?,?)", mergeID, p.SourcePersonID, name, p.TargetPersonID)
	return err
}

// mergePersonInto 把 srcID 的人物档案合并进 tgtID：补齐空字段、重指关系/族谱/照片/家谱/家族关联，最后删除 srcID。
// 返回受影响的家族 id（用于事后修复关系正反指向）、提示信息，以及被去掉的关系条数（自环或重复）。
func mergePersonInto(tx *sql.Tx, srcID, tgtID, now string) ([]string, []string, int, error) {
	warnings := []string{}
	dropped := 0
	families := map[string]bool{}
	fields := "name,gender,birth_date,death_date,birthplace,occupation,biography,COALESCE(claimed_by,'')"
	var sName, sGender, sBirth, sDeath, sBirthplace, sOccupation, sBio, sClaimed string
	if err := tx.QueryRow("SELECT "+fields+" FROM persons WHERE id=?", srcID).Scan(&sName, &sGender, &sBirth, &sDeath, &sBirthplace, &sOccupation, &sBio, &sClaimed); err != nil {
		return nil, warnings, dropped, err
	}
	var tName, tGender, tBirth, tDeath, tBirthplace, tOccupation, tBio, tClaimed string
	if err := tx.QueryRow("SELECT "+fields+" FROM persons WHERE id=?", tgtID).Scan(&tName, &tGender, &tBirth, &tDeath, &tBirthplace, &tOccupation, &tBio, &tClaimed); err != nil {
		return nil, warnings, dropped, err
	}
	pick := func(dst, src string) string {
		if strings.TrimSpace(dst) == "" {
			return src
		}
		return dst
	}
	gender := tGender
	if gender == "" || gender == "unknown" {
		gender = sGender
	}
	claim := tClaimed
	if claim == "" {
		claim = sClaimed
	} else if sClaimed != "" && sClaimed != tClaimed {
		warnings = append(warnings, fmt.Sprintf("「%s」在两边被不同账号认领，已保留接收方档案的认领关系", tName))
	}
	if _, err := tx.Exec(`UPDATE persons SET gender=?,birth_date=?,death_date=?,birthplace=?,occupation=?,biography=?,claimed_by=NULLIF(?,''),updated_at=? WHERE id=?`,
		gender, pick(tBirth, sBirth), pick(tDeath, sDeath), pick(tBirthplace, sBirthplace), pick(tOccupation, sOccupation), pick(tBio, sBio), claim, now, tgtID); err != nil {
		return nil, warnings, dropped, err
	}

	// 关系重指：两端凡是旧人物的都改成新人物；合并后自环与重复关系直接删除
	type relRow struct{ id, fam, from, to, rtype, custom string }
	rels := []relRow{}
	rows, err := tx.Query("SELECT id,family_id,from_person_id,to_person_id,relation_type,custom_name FROM person_relations WHERE from_person_id=? OR to_person_id=?", srcID, srcID)
	if err != nil {
		return nil, warnings, dropped, err
	}
	for rows.Next() {
		var r relRow
		if rows.Scan(&r.id, &r.fam, &r.from, &r.to, &r.rtype, &r.custom) == nil {
			rels = append(rels, r)
		}
	}
	rows.Close()
	for _, r := range rels {
		families[r.fam] = true
		nf, nt := r.from, r.to
		if nf == srcID {
			nf = tgtID
		}
		if nt == srcID {
			nt = tgtID
		}
		if nf == nt {
			// 两个人合成一个人后，原先「自己对自己」的关系不再成立
			if _, err := tx.Exec("DELETE FROM person_relations WHERE id=?", r.id); err != nil {
				return nil, warnings, dropped, err
			}
			dropped++
			continue
		}
		var dup int
		if tx.QueryRow(`SELECT COUNT(*) FROM person_relations WHERE family_id=? AND from_person_id=? AND to_person_id=? AND relation_type=? AND custom_name=? AND id<>?`,
			r.fam, nf, nt, r.rtype, r.custom, r.id).Scan(&dup) == nil && dup > 0 {
			if _, err := tx.Exec("DELETE FROM person_relations WHERE id=?", r.id); err != nil {
				return nil, warnings, dropped, err
			}
			dropped++
			continue
		}
		if _, err := tx.Exec("UPDATE person_relations SET from_person_id=?,to_person_id=? WHERE id=?", nf, nt, r.id); err != nil {
			return nil, warnings, dropped, err
		}
	}

	// 族谱 / 照片 / 家谱 / 家族关联改指到新人物，再删除旧人物
	if _, err := tx.Exec("INSERT OR IGNORE INTO genealogy_persons(genealogy_id,person_id) SELECT genealogy_id,? FROM genealogy_persons WHERE person_id=?", tgtID, srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("DELETE FROM genealogy_persons WHERE person_id=?", srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO photo_persons(photo_id,person_id) SELECT photo_id,? FROM photo_persons WHERE person_id=?", tgtID, srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("DELETE FROM photo_persons WHERE person_id=?", srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("UPDATE photos SET person_id=? WHERE person_id=?", tgtID, srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("UPDATE books SET root_person_id=? WHERE root_person_id=?", tgtID, srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO person_families(person_id,family_id) SELECT ?,family_id FROM person_families WHERE person_id=?", tgtID, srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("DELETE FROM person_families WHERE person_id=?", srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if _, err := tx.Exec("DELETE FROM persons WHERE id=?", srcID); err != nil {
		return nil, warnings, dropped, err
	}
	if dropped > 0 {
		warnings = append(warnings, fmt.Sprintf("「%s」合并后有 %d 条关系不再成立（自环或重复），已自动移除", tName, dropped))
	}
	out := make([]string, 0, len(families))
	for f := range families {
		out = append(out, f)
	}
	return out, warnings, dropped, nil
}

// repairInverseLinks 按「反向人物 + 反向关系类型 + 自定义名」重新配对 inverse_id，
// 与 createRelation 生成正反两行的规则保持一致；找不到配对时置空（避免悬空引用）。
func repairInverseLinks(tx *sql.Tx, families []string) error {
	seen := map[string]bool{}
	for _, fid := range families {
		if fid == "" || seen[fid] {
			continue
		}
		seen[fid] = true
		type relRow struct{ id, from, to, rtype, custom string }
		rows, err := tx.Query("SELECT id,from_person_id,to_person_id,relation_type,custom_name FROM person_relations WHERE family_id=?", fid)
		if err != nil {
			return err
		}
		all := []relRow{}
		for rows.Next() {
			var r relRow
			if rows.Scan(&r.id, &r.from, &r.to, &r.rtype, &r.custom) == nil {
				all = append(all, r)
			}
		}
		rows.Close()
		if len(all) == 0 {
			continue
		}
		index := map[string]string{}
		for _, r := range all {
			index[r.from+"|"+r.to+"|"+r.rtype+"|"+r.custom] = r.id
		}
		for _, r := range all {
			inverse := inverseRelation[r.rtype]
			if inverse == "" {
				inverse = "CUSTOM"
			}
			want := index[r.to+"|"+r.from+"|"+inverse+"|"+r.custom]
			if _, err := tx.Exec("UPDATE person_relations SET inverse_id=? WHERE id=?", want, r.id); err != nil {
				return err
			}
		}
	}
	return nil
}
