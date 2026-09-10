package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// 家族合并的端到端测试：直接跑 HTTP 层 + 直接查库校验数据一致性。
// 运行：cd Jia_api && go test ./cmd/server -run TestFamilyMerge -v

type testEnv struct {
	t     *testing.T
	app   *app
	route http.Handler
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_init.sql"))
	if err != nil {
		t.Fatalf("读取 schema 失败: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("初始化 schema 失败: %v", err)
	}
	instance := &app{db: db, jwtSecret: []byte("test-secret"), storageDir: dir}
	if err := ensureMergeSchema(db); err != nil {
		t.Fatalf("初始化合并表失败: %v", err)
	}
	return &testEnv{t: t, app: instance, route: instance.routes()}
}

func (e *testEnv) call(method, path, token string, body any) (int, map[string]any) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.route.ServeHTTP(rec, req)
	parsed := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			e.t.Fatalf("%s %s 响应不是合法 JSON: %s", method, path, rec.Body.String())
		}
	}
	return rec.Code, parsed
}

func (e *testEnv) ok(method, path, token string, body any) map[string]any {
	e.t.Helper()
	status, res := e.call(method, path, token, body)
	if status < 200 || status >= 300 {
		e.t.Fatalf("%s %s 期望成功，实际 %d：%v", method, path, status, res["error"])
	}
	data, _ := res["data"].(map[string]any)
	return data
}

func (e *testEnv) fail(method, path, token string, body any, want int) {
	e.t.Helper()
	status, res := e.call(method, path, token, body)
	if status != want {
		e.t.Fatalf("%s %s 期望状态码 %d，实际 %d：%v", method, path, want, status, res)
	}
}

func (e *testEnv) list(method, path, token string) []any {
	e.t.Helper()
	status, res := e.call(method, path, token, nil)
	if status != 200 {
		e.t.Fatalf("%s %s 期望 200，实际 %d：%v", method, path, status, res["error"])
	}
	out, _ := res["data"].([]any)
	return out
}

func (e *testEnv) scalar(query string, args ...any) string {
	e.t.Helper()
	var value sql.NullString
	if err := e.app.db.QueryRow(query, args...).Scan(&value); err != nil {
		e.t.Fatalf("查询失败 %q: %v", query, err)
	}
	return value.String
}

func field(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, _ := m[key].(string)
	return v
}

func number(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	if v, ok := m[key].(float64); ok {
		return v
	}
	t.Fatalf("字段 %s 不是数字：%v", key, m[key])
	return 0
}

func TestFamilyMerge(t *testing.T) {
	env := newTestEnv(t)

	// ---------- 准备：两个 owner 各自建了「同一个家族」，还有一个被邀请的成员 ----------
	aToken := field(t, env.ok("POST", "/api/v1/auth/register", "", map[string]any{"email": "owner-a@example.com", "password": "secret123", "displayName": "甲方"}), "token")
	bToken := field(t, env.ok("POST", "/api/v1/auth/register", "", map[string]any{"email": "owner-b@example.com", "password": "secret123", "displayName": "乙方"}), "token")
	cToken := field(t, env.ok("POST", "/api/v1/auth/register", "", map[string]any{"email": "member-c@example.com", "password": "secret123", "displayName": "丙方"}), "token")

	familyA := field(t, env.ok("POST", "/api/v1/families", aToken, map[string]any{"name": "徐氏家族", "surname": "徐"}), "id")
	familyB := field(t, env.ok("POST", "/api/v1/families", bToken, map[string]any{"name": "徐氏族谱（乙方）", "surname": "徐"}), "id")

	person := func(token, family, name, gender, birth string) string {
		return field(t, env.ok("POST", "/api/v1/families/"+family+"/persons", token, map[string]any{"name": name, "gender": gender, "birthDate": birth}), "id")
	}
	relate := func(token, from, to, kind, familyID string) {
		env.ok("POST", "/api/v1/persons/"+from+"/relations", token, map[string]any{"toPersonID": to, "relationType": kind, "familyId": familyID})
	}

	// A 家族：祖父 / 徐书帆 / 张峪宁
	fuA := person(aToken, familyA, "徐父", "male", "1950-01-01")
	shuA := person(aToken, familyA, "徐书帆", "male", "1980-05-06")
	ningA := person(aToken, familyA, "张峪宁", "female", "1985-08-08")
	relate(aToken, fuA, shuA, "FATHER", "")
	relate(aToken, shuA, ningA, "HUSBAND", "")

	// B 家族：同一个徐书帆（重复）+ 儿子徐小明
	shuB := person(bToken, familyB, "徐书帆", "male", "1980-05-06")
	mingB := person(bToken, familyB, "徐小明", "male", "2010-01-01")
	relate(bToken, shuB, mingB, "FATHER", "")

	// 两边同名的族谱
	genA := field(t, env.ok("POST", "/api/v1/families/"+familyA+"/genealogies", aToken, map[string]any{"name": "徐氏族谱"}), "id")
	genB := field(t, env.ok("POST", "/api/v1/families/"+familyB+"/genealogies", bToken, map[string]any{"name": "徐氏族谱"}), "id")
	env.ok("POST", "/api/v1/genealogies/"+genA+"/persons", aToken, map[string]any{"person_id": shuA})
	env.ok("POST", "/api/v1/genealogies/"+genB+"/persons", bToken, map[string]any{"person_id": shuB})
	env.ok("POST", "/api/v1/genealogies/"+genB+"/persons", bToken, map[string]any{"person_id": mingB})

	// B 家族有一部以「徐书帆」为根的家谱
	bookB := field(t, env.ok("POST", "/api/v1/families/"+familyB+"/books", bToken, map[string]any{"title": "徐书帆生平", "rootPersonID": shuB}), "id")

	// 乙邀请丙加入 B 家族（editor），验证成员也会被带过去
	invite := field(t, env.ok("POST", "/api/v1/families/"+familyB+"/invites", bToken, map[string]any{"email": "member-c@example.com", "role": "editor"}), "token")
	env.ok("POST", "/api/v1/invites/"+invite+"/accept", cToken, nil)

	// 制造「关系重复」边界：合并前双方互相把对方拉进自己家族（现实中常见），
	// 让同一条 徐父→徐小明 的关系在 A、B 两个家族里各存一份，合并时不能撞唯一约束。
	inviteA := field(t, env.ok("POST", "/api/v1/families/"+familyA+"/invites", aToken, map[string]any{"email": "owner-b@example.com", "role": "editor"}), "token")
	env.ok("POST", "/api/v1/invites/"+inviteA+"/accept", bToken, nil)
	env.ok("POST", "/api/v1/persons/"+mingB+"/families", bToken, map[string]any{"family_id": familyA})
	env.ok("POST", "/api/v1/persons/"+fuA+"/families", bToken, map[string]any{"family_id": familyB})
	relate(aToken, fuA, mingB, "FATHER", familyA)
	relate(bToken, fuA, mingB, "FATHER", familyB)

	// ---------- 1. 权限：非 owner 不能生成合并码 ----------
	env.fail("POST", "/api/v1/families/"+familyA+"/merge-invites", cToken, nil, 403)
	env.fail("POST", "/api/v1/families/"+familyB+"/merge-invites", cToken, nil, 403)

	// ---------- 2. 主家族 A 生成合并码 ----------
	code := env.ok("POST", "/api/v1/families/"+familyA+"/merge-invites", aToken, nil)
	token := field(t, code, "token")
	if len(token) != 14 {
		t.Fatalf("合并码格式异常：%q", token)
	}

	// 合并码大小写/连字符容错
	lookup := env.ok("GET", "/api/v1/merge-invites/"+token, bToken, nil)
	if target, _ := lookup["target_family"].(map[string]any); field(t, target, "id") != familyA {
		t.Fatalf("合并码指向的家族不对：%v", lookup["target_family"])
	}
	messy := token[0:4] + token[5:9] + token[10:]
	env.ok("GET", "/api/v1/merge-invites/"+messy, bToken, nil)

	// ---------- 3. 预览：只有 B 的 owner 能预览，且必须给出建议配对 ----------
	env.fail("POST", "/api/v1/merge-invites/"+token+"/preview", cToken, map[string]any{"source_family_id": familyB}, 403)
	env.fail("POST", "/api/v1/merge-invites/"+token+"/preview", bToken, map[string]any{"source_family_id": familyA}, 400)

	preview := env.ok("POST", "/api/v1/merge-invites/"+token+"/preview", bToken, map[string]any{"source_family_id": familyB})
	suggestions, _ := preview["suggestions"].([]any)
	if len(suggestions) != 1 {
		t.Fatalf("期望 1 条人物配对建议，实际 %d：%v", len(suggestions), suggestions)
	}
	suggestion, _ := suggestions[0].(map[string]any)
	if field(t, suggestion, "source_person_id") != shuB || field(t, suggestion, "target_person_id") != shuA {
		t.Fatalf("配对建议不对：%v", suggestion)
	}
	if confidence := field(t, suggestion, "confidence"); confidence != "high" {
		t.Fatalf("同名同生日应为 high 置信度，实际 %q", confidence)
	}
	sameName, _ := preview["same_name_genealogies"].([]any)
	if len(sameName) != 1 {
		t.Fatalf("期望识别出 1 份同名族谱，实际 %d", len(sameName))
	}

	// ---------- 4. 非法人物对照要被拒绝 ----------
	env.fail("POST", "/api/v1/merge-invites/"+token+"/execute", bToken, map[string]any{
		"source_family_id": familyB, "pairs": []map[string]string{{"source_person_id": shuA, "target_person_id": shuA}},
	}, 400)
	env.fail("POST", "/api/v1/merge-invites/"+token+"/execute", bToken, map[string]any{
		"source_family_id": familyB, "pairs": []map[string]string{{"source_person_id": mingB, "target_person_id": ningA}, {"source_person_id": shuB, "target_person_id": ningA}},
	}, 400)

	// ---------- 5. 执行合并 ----------
	summary := env.ok("POST", "/api/v1/merge-invites/"+token+"/execute", bToken, map[string]any{
		"source_family_id":            familyB,
		"pairs":                       []map[string]string{{"source_person_id": shuB, "target_person_id": shuA}},
		"merge_same_name_genealogies": true,
	})
	if got := number(t, summary, "persons_merged"); got != 1 {
		t.Fatalf("期望合并 1 位重复人物，实际 %v", got)
	}
	// 合并结果里带回两边的家族信息（前端结果页要展示 来源 → 目标）
	if src, _ := summary["source_family"].(map[string]any); field(t, src, "name") != "徐氏族谱（乙方）" {
		t.Fatalf("结果里缺少被并入家族信息：%v", summary["source_family"])
	}
	if tgt, _ := summary["target_family"].(map[string]any); field(t, tgt, "name") != "徐氏家族" {
		t.Fatalf("结果里缺少目标家族信息：%v", summary["target_family"])
	}

	// ---------- 6. 合并码只能用一次 ----------
	env.fail("POST", "/api/v1/merge-invites/"+token+"/execute", bToken, map[string]any{"source_family_id": familyB}, 409)
	env.fail("GET", "/api/v1/merge-invites/"+token, bToken, nil, 409)

	// ---------- 7. 家族列表：B 家族消失，B 的 owner 成为 A 的 owner ----------
	familiesB := env.list("GET", "/api/v1/families", bToken)
	if len(familiesB) != 1 {
		t.Fatalf("乙方合并后应只剩 1 个家族，实际 %d：%v", len(familiesB), familiesB)
	}
	remaining, _ := familiesB[0].(map[string]any)
	if field(t, remaining, "id") != familyA {
		t.Fatalf("乙方合并后应看到 A 家族，实际 %v", remaining)
	}
	familiesC := env.list("GET", "/api/v1/families", cToken)
	if len(familiesC) != 1 {
		t.Fatalf("丙方应只保留 A 家族，实际 %d", len(familiesC))
	}
	roleOf := func(family, email string) string {
		for _, item := range env.list("GET", "/api/v1/families/"+family+"/members", aToken) {
			member, _ := item.(map[string]any)
			if field(t, member, "email") == email {
				return field(t, member, "role")
			}
		}
		return ""
	}
	if role := roleOf(familyA, "owner-b@example.com"); role != "owner" {
		t.Fatalf("被并入方的 owner 应保留 owner 权限，实际 %q", role)
	}
	if role := roleOf(familyA, "member-c@example.com"); role != "editor" {
		t.Fatalf("B 家族的 editor 成员应并入 A，实际 %q", role)
	}
	if role := roleOf(familyA, "owner-a@example.com"); role != "owner" {
		t.Fatalf("主家族 owner 权限不应受影响，实际 %q", role)
	}

	// ---------- 8. 人物：4 位（重复的徐书帆已合并），旧 id 不再存在 ----------
	persons := env.list("GET", "/api/v1/families/"+familyA+"/persons", aToken)
	if len(persons) != 4 {
		t.Fatalf("合并后 A 家族应有 4 位人物，实际 %d：%v", len(persons), persons)
	}
	names := map[string]bool{}
	for _, item := range persons {
		person, _ := item.(map[string]any)
		names[field(t, person, "name")] = true
		if field(t, person, "id") == shuB {
			t.Fatal("被合并掉的重复人物仍然存在")
		}
	}
	for _, want := range []string{"徐父", "徐书帆", "张峪宁", "徐小明"} {
		if !names[want] {
			t.Fatalf("合并后缺少人物 %s", want)
		}
	}
	if env.scalar("SELECT COUNT(*) FROM persons WHERE family_id=?", familyB) != "0" {
		t.Fatal("被并入家族仍残留人物")
	}

	// ---------- 9. 关系：全部改指到合并后的人物，且没有自环 ----------
	edges := func(center string) []any {
		data := env.ok("GET", "/api/v1/persons/"+center+"/graph", aToken, nil)
		list, _ := data["edges"].([]any)
		return list
	}
	neighbours := map[string]string{}
	for _, item := range edges(shuA) {
		edge, _ := item.(map[string]any)
		from, to := field(t, edge, "from_person_id"), field(t, edge, "to_person_id")
		other := to
		if from == shuA {
			other = to
		} else {
			other = from
		}
		if from == to {
			t.Fatalf("出现了自环关系：%v", edge)
		}
		neighbours[other] = field(t, edge, "relation_type")
	}
	if len(neighbours) != 3 {
		t.Fatalf("徐书帆合并后应有 3 个关系对象（徐父/张峪宁/徐小明），实际 %d：%v", len(neighbours), neighbours)
	}
	for _, want := range []string{fuA, ningA, mingB} {
		if _, ok := neighbours[want]; !ok {
			t.Fatalf("徐书帆缺少与 %s 的关系", want)
		}
	}
	if env.scalar("SELECT COUNT(*) FROM person_relations WHERE family_id=?", familyB) != "0" {
		t.Fatal("被并入家族仍残留关系")
	}
	if n := env.scalar("SELECT COUNT(*) FROM person_relations WHERE from_person_id=to_person_id"); n != "0" {
		t.Fatalf("存在 %s 条自环关系", n)
	}

	// 正反指向完整性：每条关系的 inverse_id 必须指回反向的那条
	var dangling int
	rows, err := env.app.db.Query(`SELECT COUNT(*) FROM person_relations r
		WHERE r.family_id=? AND (r.inverse_id='' OR NOT EXISTS (
			SELECT 1 FROM person_relations i WHERE i.id=r.inverse_id
			  AND i.from_person_id=r.to_person_id AND i.to_person_id=r.from_person_id))`, familyA)
	if err != nil {
		t.Fatalf("校验正反指向失败: %v", err)
	}
	if rows.Next() {
		if err := rows.Scan(&dangling); err != nil {
			t.Fatalf("读取校验结果失败: %v", err)
		}
	}
	rows.Close()
	if dangling != 0 {
		t.Fatalf("有 %d 条关系的正反指向断裂", dangling)
	}

	// ---------- 10. 族谱合并、家谱根人物改指 ----------
	genealogies := env.list("GET", "/api/v1/families/"+familyA+"/genealogies", aToken)
	if len(genealogies) != 1 {
		t.Fatalf("同名族谱应合并为 1 份，实际 %d", len(genealogies))
	}
	genealogyPersons := env.list("GET", "/api/v1/genealogies/"+genA+"/persons", aToken)
	if len(genealogyPersons) != 2 {
		t.Fatalf("合并后的族谱应包含 2 位人物（徐书帆、徐小明；重复的徐书帆已并成一位），实际 %d", len(genealogyPersons))
	}
	mergedGenealogy := map[string]bool{}
	for _, item := range genealogyPersons {
		p, _ := item.(map[string]any)
		mergedGenealogy[field(t, p, "name")] = true
	}
	if !mergedGenealogy["徐书帆"] || !mergedGenealogy["徐小明"] {
		t.Fatalf("合并后的族谱人物不对：%v", mergedGenealogy)
	}
	if env.scalar("SELECT COUNT(*) FROM genealogy_persons WHERE genealogy_id=?", genB) != "0" {
		t.Fatal("被并入家族的同名族谱仍残留人物关联")
	}
	if env.scalar("SELECT COUNT(*) FROM genealogies WHERE family_id=?", familyB) != "0" {
		t.Fatal("被并入家族仍残留族谱")
	}
	if root := env.scalar("SELECT COALESCE(root_person_id,'') FROM books WHERE id=?", bookB); root != shuA {
		t.Fatalf("家谱根人物应改指到合并后的人物，实际 %q（期望 %q）", root, shuA)
	}
	if family := env.scalar("SELECT family_id FROM books WHERE id=?", bookB); family != familyA {
		t.Fatalf("家谱未迁入目标家族：%q", family)
	}

	// ---------- 11. 审计与动态 ----------
	if got := env.scalar("SELECT source_family_id FROM family_merges WHERE token=?", token); got != familyB {
		t.Fatalf("合并记录里的被并入家族不对：%q", got)
	}
	if got := env.scalar("SELECT target_person_id FROM family_merge_pairs WHERE source_person_id=?", shuB); got != shuA {
		t.Fatalf("人物对照记录不对：%q", got)
	}
	if env.scalar("SELECT COUNT(*) FROM activities WHERE family_id=? AND type='family.merged'", familyA) != "1" {
		t.Fatal("目标家族缺少合并动态记录")
	}
	if env.scalar("SELECT COUNT(*) FROM family_members WHERE family_id=?", familyB) != "0" {
		t.Fatal("被并入家族仍残留成员")
	}

	// ---------- 12. 合并后的关系图仍然可用（无悬空节点） ----------
	graph := env.ok("GET", "/api/v1/families/"+familyA+"/graph", aToken, nil)
	nodes, _ := graph["nodes"].([]any)
	graphEdges, _ := graph["edges"].([]any)
	if len(nodes) != 4 {
		t.Fatalf("关系图应有 4 个节点，实际 %d", len(nodes))
	}
	if len(graphEdges) != 8 {
		t.Fatalf("关系图应有 8 条边（徐父↔徐书帆、徐书帆↔张峪宁、徐父↔徐小明、徐书帆↔徐小明 四对正反），实际 %d", len(graphEdges))
	}
	fmt.Printf("合并完成：人物 %v 位、关系 %v 条、族谱合并 %v 份\n", summary["person_count"], summary["relations_moved"], summary["genealogies_merged"])
}
