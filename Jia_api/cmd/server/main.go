package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type contextKey string

const userIDKey contextKey = "userID"

type app struct {
	db         *sql.DB
	jwtSecret  []byte
	storageDir string
}

type user struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   string `json:"created_at"`
}

type family struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Surname          string `json:"surname"`
	Origin           string `json:"origin"`
	MigrationHistory string `json:"migration_history"`
	Creed            string `json:"creed"`
	Description      string `json:"description"`
	CreatedBy        string `json:"created_by"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	PersonCount      int    `json:"person_count,omitempty"`
	GenealogyCount   int    `json:"genealogy_count,omitempty"`
	MemberCount      int    `json:"member_count,omitempty"`
}

type genealogy struct {
	ID          string `json:"id"`
	FamilyID    string `json:"family_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	PersonCount int    `json:"person_count,omitempty"`
}

type person struct {
	ID         string `json:"id"`
	FamilyID   string `json:"family_id"`
	Name       string `json:"name"`
	Gender     string `json:"gender"`
	BirthDate  string `json:"birth_date"`
	DeathDate  string `json:"death_date"`
	Birthplace string `json:"birthplace"`
	Occupation string `json:"occupation"`
	Biography  string `json:"biography"`
	ClaimedBy  string `json:"claimed_by"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type relation struct {
	ID           string `json:"id"`
	FromPersonID string `json:"from_person_id"`
	ToPersonID   string `json:"to_person_id"`
	RelationType string `json:"relation_type"`
	CustomName   string `json:"custom_name"`
	Note         string `json:"note"`
}

type book struct {
	ID           string `json:"id"`
	FamilyID     string `json:"family_id"`
	Title        string `json:"title"`
	RootPersonID string `json:"root_person_id"`
	ContentJSON  string `json:"content_json"`
	CreatedBy    string `json:"created_by"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type photo struct {
	ID           string `json:"id"`
	FamilyID     string `json:"family_id"`
	PersonID     string `json:"person_id"`
	Path         string `json:"path"`
	OriginalName string `json:"original_name"`
	Caption      string `json:"caption"`
	Category     string `json:"category"`
	TakenAt      string `json:"taken_at"`
	Location     string `json:"location"`
	CreatedAt    string `json:"created_at"`
}

func main() {
	dbPath := getenv("JIA_DB_PATH", filepath.Join("data", "jia.db"))
	storageDir := getenv("JIA_STORAGE_DIR", "storage")
	secret := getenv("JIA_JWT_SECRET", "jia-development-secret-change-me")
	addr := getenv("JIA_ADDR", ":8081")
	// 默认数据路径相对当前工作目录解析，启动时记录绝对路径，避免在错误目录启动导致连到空库。
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	if abs, err := filepath.Abs(storageDir); err == nil {
		storageDir = abs
	}
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Fatal(err)
	}
	var userTable int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userTable); err != nil {
		log.Fatalf("数据库 %s 缺少 users 表（schema 未初始化）。请在 Jia_api 目录下执行: go run ./cmd/migrate 后重试", dbPath)
	}
	a := &app{db: db, jwtSecret: []byte(secret), storageDir: storageDir}
	log.Printf("数据文件: %s", dbPath)
	log.Printf("存储目录: %s", storageDir)
	log.Printf("Jia API listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, a.routes()))
}

func (a *app) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, cors)
	r.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir(a.storageDir))))
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.register)
		r.Post("/auth/login", a.login)
		r.Post("/auth/forgot-password", a.forgotPassword)
		r.Post("/auth/reset-password", a.resetPassword)
		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Get("/auth/me", a.me)
			r.Get("/families", a.listFamilies)
			r.Post("/families", a.createFamily)
			r.Get("/families/{familyID}", a.getFamily)
			r.Patch("/families/{familyID}", a.updateFamily)
			r.Delete("/families/{familyID}", a.deleteFamily)
			r.Get("/families/{familyID}/members", a.listMembers)
			r.Post("/families/{familyID}/members", a.addMember)
			r.Patch("/members/{memberID}", a.updateMember)
			r.Delete("/members/{memberID}", a.deleteMember)
			r.Post("/families/{familyID}/invites", a.createInvite)
			r.Post("/invites/{token}/accept", a.acceptInvite)
			r.Get("/families/{familyID}/genealogies", a.listGenealogies)
			r.Post("/families/{familyID}/genealogies", a.createGenealogy)
			r.Get("/genealogies/{genealogyID}", a.getGenealogy)
			r.Patch("/genealogies/{genealogyID}", a.updateGenealogy)
			r.Delete("/genealogies/{genealogyID}", a.deleteGenealogy)
			r.Get("/genealogies/{genealogyID}/persons", a.listGenealogyPersons)
			r.Post("/genealogies/{genealogyID}/persons", a.addGenealogyPerson)
			r.Delete("/genealogies/{genealogyID}/persons/{personID}", a.removeGenealogyPerson)
			r.Get("/families/{familyID}/persons", a.listPersons)
			r.Post("/families/{familyID}/persons", a.createPerson)
			r.Get("/persons/{personID}", a.getPerson)
			r.Patch("/persons/{personID}", a.updatePerson)
			r.Delete("/persons/{personID}", a.deletePerson)
			r.Get("/persons/{personID}/genealogies", a.listPersonGenealogies)
			r.Get("/persons/{personID}/books", a.listPersonBooks)
			r.Get("/persons/{personID}/photos", a.listPersonPhotos)
			r.Post("/persons/{personID}/relations", a.createRelation)
			r.Patch("/relations/{relationID}", a.updateRelation)
			r.Delete("/relations/{relationID}", a.deleteRelation)
			r.Get("/persons/{personID}/graph", a.personGraph)
			r.Get("/families/{familyID}/graph", a.familyGraph)
			r.Get("/families/{familyID}/books", a.listBooks)
			r.Post("/families/{familyID}/books", a.createBook)
			r.Get("/books/{bookID}", a.getBook)
			r.Patch("/books/{bookID}", a.updateBook)
			r.Delete("/books/{bookID}", a.deleteBook)
			r.Get("/books/{bookID}/collaborators", a.listBookCollaborators)
			r.Post("/books/{bookID}/collaborators", a.addBookCollaborator)
			r.Delete("/books/{bookID}/collaborators/{userID}", a.removeBookCollaborator)
			r.Post("/persons/{personID}/photos", a.uploadPhoto)
			r.Get("/families/{familyID}/photos", a.listPhotos)
			r.Get("/photos/{photoID}", a.getPhoto)
			r.Patch("/photos/{photoID}", a.updatePhoto)
			r.Delete("/photos/{photoID}", a.deletePhoto)
			r.Get("/photos/{photoID}/persons", a.listPhotoPersons)
			r.Post("/photos/{photoID}/persons", a.addPhotoPerson)
			r.Delete("/photos/{photoID}/persons/{personID}", a.removePhotoPerson)
			r.Post("/persons/{personID}/claim", a.claimPerson)
			r.Post("/persons/{personID}/move", a.movePerson)
			r.Get("/persons/{personID}/families", a.personFamilies)
			r.Post("/persons/{personID}/families", a.linkPersonFamily)
			r.Delete("/persons/{personID}/families/{familyID}", a.unlinkPersonFamily)
			r.Get("/families/{familyID}/activities", a.activities)
		})
	})
	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if strings.HasPrefix(origin, "http://") && strings.HasSuffix(origin, ":5173") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		token, err := jwt.Parse(h, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return a.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			errorJSON(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		id, okID := claims["sub"].(string)
		if !ok || !okID || id == "" {
			errorJSON(w, http.StatusUnauthorized, "unauthorized", "登录凭证无效")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, id)))
	})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password, DisplayName string }
	if !decode(r, &in) || strings.TrimSpace(in.Email) == "" || len(in.Password) < 6 || strings.TrimSpace(in.DisplayName) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请填写有效的邮箱、至少六位密码和昵称")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "hash_failed", "密码处理失败")
		return
	}
	now, id := time.Now().UTC().Format(time.RFC3339), newID()
	_, err = a.db.Exec("INSERT INTO users(id,email,password_hash,display_name,created_at) VALUES(?,?,?,?,?)", id, strings.ToLower(strings.TrimSpace(in.Email)), string(hash), strings.TrimSpace(in.DisplayName), now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			errorJSON(w, http.StatusConflict, "email_exists", "邮箱已注册")
		} else {
			errorJSON(w, http.StatusInternalServerError, "database_error", "注册失败，请稍后重试")
		}
		return
	}
	a.authResponse(w, id, http.StatusCreated)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !decode(r, &in) {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请求参数无效")
		return
	}
	var id, hash string
	if err := a.db.QueryRow("SELECT id,password_hash FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&id, &hash); err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		errorJSON(w, http.StatusUnauthorized, "invalid_credentials", "邮箱或密码错误")
		return
	}
	a.authResponse(w, id, http.StatusOK)
}

func (a *app) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.Email) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请输入邮箱")
		return
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	response := map[string]any{
		"message":     "如果邮箱已注册，重置凭证已生成",
		"reset_token": "",
		"expires_at":  "",
	}
	var userID string
	if err := a.db.QueryRow("SELECT id FROM users WHERE email=?", email).Scan(&userID); err == nil {
		now := time.Now().UTC()
		expiresAt := now.Add(30 * time.Minute).Format(time.RFC3339)
		token := newID() + newID()
		tokenHash := hashResetToken(token)
		tx, err := a.db.Begin()
		if err != nil {
			errorJSON(w, http.StatusInternalServerError, "database_error", "生成重置凭证失败")
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE password_reset_tokens SET used_at=? WHERE user_id=? AND used_at=''", now.Format(time.RFC3339), userID); err != nil {
			errorJSON(w, http.StatusInternalServerError, "database_error", "生成重置凭证失败")
			return
		}
		if _, err = tx.Exec("INSERT INTO password_reset_tokens(id,user_id,token_hash,expires_at,used_at,created_at) VALUES(?,?,?,?,?,?)", newID(), userID, tokenHash, expiresAt, "", now.Format(time.RFC3339)); err != nil {
			errorJSON(w, http.StatusInternalServerError, "database_error", "生成重置凭证失败")
			return
		}
		if err = tx.Commit(); err != nil {
			errorJSON(w, http.StatusInternalServerError, "database_error", "生成重置凭证失败")
			return
		}
		response["reset_token"] = token
		response["expires_at"] = expiresAt
	}
	jsonResponse(w, http.StatusOK, response)
}

func (a *app) resetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.Token) == "" || len(in.Password) < 6 {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请输入重置凭证和至少六位新密码")
		return
	}

	now := time.Now().UTC()
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "重置密码失败")
		return
	}
	defer tx.Rollback()
	var tokenID, userID, expiresAt, usedAt string
	err = tx.QueryRow("SELECT id,user_id,expires_at,used_at FROM password_reset_tokens WHERE token_hash=?", hashResetToken(strings.TrimSpace(in.Token))).Scan(&tokenID, &userID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows || usedAt != "" {
		errorJSON(w, http.StatusBadRequest, "reset_token_invalid", "重置凭证无效")
		return
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取重置凭证失败")
		return
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "重置凭证数据无效")
		return
	}
	if !now.Before(expires) {
		errorJSON(w, http.StatusGone, "reset_token_expired", "重置凭证已过期")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "hash_failed", "密码处理失败")
		return
	}
	if _, err = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), userID); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "重置密码失败")
		return
	}
	result, err := tx.Exec("UPDATE password_reset_tokens SET used_at=? WHERE id=? AND used_at=''", now.Format(time.RFC3339), tokenID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "重置凭证失效失败")
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		errorJSON(w, http.StatusBadRequest, "reset_token_invalid", "重置凭证无效")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "重置密码失败")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"reset": true})
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *app) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.findUser(userID(r.Context()))
	if err != nil {
		errorJSON(w, http.StatusNotFound, "user_not_found", "用户不存在")
		return
	}
	jsonResponse(w, http.StatusOK, u)
}

func (a *app) authResponse(w http.ResponseWriter, id string, status int) {
	u, _ := a.findUser(id)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": id, "exp": time.Now().Add(24 * time.Hour).Unix()})
	token, _ := t.SignedString(a.jwtSecret)
	jsonResponse(w, status, map[string]any{"token": token, "user": u})
}

func (a *app) listFamilies(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT f.id,f.name,f.surname,f.origin,f.migration_history,f.creed,f.description,f.created_by,f.created_at,f.updated_at,
	 (SELECT COUNT(*) FROM persons p WHERE p.family_id=f.id),(SELECT COUNT(*) FROM genealogies g WHERE g.family_id=f.id),(SELECT COUNT(*) FROM family_members m WHERE m.family_id=f.id)
	 FROM families f JOIN family_members m ON m.family_id=f.id WHERE m.user_id=? ORDER BY f.updated_at DESC`, userID(r.Context()))
	if err != nil {
		errorJSON(w, 500, "database_error", "读取家族失败")
		return
	}
	defer rows.Close()
	out := []family{}
	for rows.Next() {
		var f family
		if rows.Scan(&f.ID, &f.Name, &f.Surname, &f.Origin, &f.MigrationHistory, &f.Creed, &f.Description, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt, &f.PersonCount, &f.GenealogyCount, &f.MemberCount) == nil {
			out = append(out, f)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) createFamily(w http.ResponseWriter, r *http.Request) {
	var in struct{ Name, Surname, Origin, MigrationHistory, Creed, Description string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		errorJSON(w, 400, "invalid_input", "家族名称不能为空")
		return
	}
	now, id := time.Now().UTC().Format(time.RFC3339), newID()
	uid := userID(r.Context())
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, 500, "database_error", "创建家族失败")
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO families(id,name,surname,origin,migration_history,creed,description,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)", id, strings.TrimSpace(in.Name), in.Surname, in.Origin, in.MigrationHistory, in.Creed, in.Description, uid, now, now); err != nil {
		errorJSON(w, 500, "database_error", "创建家族失败")
		return
	}
	if _, err = tx.Exec("INSERT INTO family_members(id,family_id,user_id,role,joined_at) VALUES(?,?,?,?,?)", newID(), id, uid, "owner", now); err != nil {
		errorJSON(w, 500, "database_error", "创建家族失败")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, 500, "database_error", "创建家族失败")
		return
	}
	a.activity(id, uid, "family.created", "family", id, "创建了家族")
	a.getFamilyByID(w, id)
}

func (a *app) getFamily(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, id) {
		return
	}
	a.getFamilyByID(w, id)
}
func (a *app) getFamilyByID(w http.ResponseWriter, id string) {
	var f family
	err := a.db.QueryRow(`SELECT f.id,f.name,f.surname,f.origin,f.migration_history,f.creed,f.description,f.created_by,f.created_at,f.updated_at,(SELECT COUNT(*) FROM persons WHERE family_id=f.id),(SELECT COUNT(*) FROM genealogies WHERE family_id=f.id),(SELECT COUNT(*) FROM family_members WHERE family_id=f.id) FROM families f WHERE f.id=?`, id).Scan(&f.ID, &f.Name, &f.Surname, &f.Origin, &f.MigrationHistory, &f.Creed, &f.Description, &f.CreatedBy, &f.CreatedAt, &f.UpdatedAt, &f.PersonCount, &f.GenealogyCount, &f.MemberCount)
	if err != nil {
		errorJSON(w, 404, "not_found", "家族不存在")
		return
	}
	jsonResponse(w, 200, f)
}
func (a *app) updateFamily(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, id, "owner") {
		return
	}
	var in struct{ Name, Surname, Origin, MigrationHistory, Creed, Description string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		errorJSON(w, 400, "invalid_input", "请求参数无效")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.db.Exec("UPDATE families SET name=?,surname=?,origin=?,migration_history=?,creed=?,description=?,updated_at=? WHERE id=?", strings.TrimSpace(in.Name), strings.TrimSpace(in.Surname), strings.TrimSpace(in.Origin), strings.TrimSpace(in.MigrationHistory), strings.TrimSpace(in.Creed), strings.TrimSpace(in.Description), now, id)
	if err != nil {
		errorJSON(w, 500, "database_error", "更新家族失败")
		return
	}
	a.activity(id, userID(r.Context()), "family.updated", "family", id, "更新了家族资料")
	a.getFamilyByID(w, id)
}
func (a *app) deleteFamily(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, id, "owner") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM families WHERE id=?", id); err != nil {
		errorJSON(w, 500, "database_error", "删除家族失败")
		return
	}
	jsonResponse(w, 200, map[string]any{"deleted": true})
}

func (a *app) listMembers(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT m.id,m.user_id,u.email,u.display_name,u.avatar_url,m.role,m.status,m.joined_at FROM family_members m JOIN users u ON u.id=m.user_id WHERE m.family_id=? ORDER BY m.joined_at`, fid)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取成员失败")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, uid, email, name, avatar, role, status, joined string
		if rows.Scan(&id, &uid, &email, &name, &avatar, &role, &status, &joined) == nil {
			out = append(out, map[string]any{"id": id, "user_id": uid, "email": email, "display_name": name, "avatar_url": avatar, "role": role, "status": status, "joined_at": joined})
		}
	}
	jsonResponse(w, 200, out)
}
func (a *app) addMember(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	var in struct{ Email, Role string }
	if !decode(r, &in) || in.Email == "" {
		errorJSON(w, 400, "invalid_input", "请输入成员邮箱")
		return
	}
	if in.Role != "editor" && in.Role != "viewer" {
		in.Role = "viewer"
	}
	var uid string
	if a.db.QueryRow("SELECT id FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(in.Email))).Scan(&uid) != nil {
		errorJSON(w, 404, "user_not_found", "该邮箱尚未注册")
		return
	}
	_, err := a.db.Exec("INSERT INTO family_members(id,family_id,user_id,role,joined_at) VALUES(?,?,?,?,?)", newID(), fid, uid, in.Role, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		errorJSON(w, 409, "member_exists", "成员已存在或无法加入")
		return
	}
	a.activity(fid, userID(r.Context()), "member.added", "member", uid, "添加了家族成员")
	jsonResponse(w, 201, map[string]any{"added": true})
}
func (a *app) updateMember(w http.ResponseWriter, r *http.Request) {
	mid := chi.URLParam(r, "memberID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM family_members WHERE id=?", mid).Scan(&fid) != nil {
		errorJSON(w, 404, "not_found", "成员不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	var in struct{ Role string }
	if !decode(r, &in) || (in.Role != "owner" && in.Role != "editor" && in.Role != "viewer") {
		errorJSON(w, 400, "invalid_role", "角色无效")
		return
	}
	if _, err := a.db.Exec("UPDATE family_members SET role=? WHERE id=?", in.Role, mid); err != nil {
		errorJSON(w, 500, "database_error", "更新角色失败")
		return
	}
	var uid, email, displayName, avatarURL, status, joinedAt string
	if a.db.QueryRow(`SELECT m.user_id,u.email,u.display_name,u.avatar_url,m.status,m.joined_at
		FROM family_members m JOIN users u ON u.id=m.user_id WHERE m.id=?`, mid).Scan(&uid, &email, &displayName, &avatarURL, &status, &joinedAt) != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取更新后的成员失败")
		return
	}
	a.activity(fid, userID(r.Context()), "member.updated", "member", mid, "更新了成员角色")
	jsonResponse(w, 200, map[string]any{"id": mid, "user_id": uid, "email": email, "display_name": displayName, "avatar_url": avatarURL, "role": in.Role, "status": status, "joined_at": joinedAt})
}
func (a *app) deleteMember(w http.ResponseWriter, r *http.Request) {
	mid := chi.URLParam(r, "memberID")
	var fid, uid, role string
	if a.db.QueryRow("SELECT family_id,user_id,role FROM family_members WHERE id=?", mid).Scan(&fid, &uid, &role) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "成员不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	if role == "owner" {
		errorJSON(w, http.StatusBadRequest, "owner_required", "不能移除家族所有者")
		return
	}
	if _, err := a.db.Exec("DELETE FROM family_members WHERE id=?", mid); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移除成员失败")
		return
	}
	a.activity(fid, userID(r.Context()), "member.removed", "member", uid, "移除了家族成员")
	jsonResponse(w, http.StatusOK, map[string]any{"deleted": true})
}
func (a *app) createInvite(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	var in struct{ Email, Role string }
	if !decode(r, &in) || in.Email == "" {
		errorJSON(w, 400, "invalid_input", "请输入邮箱")
		return
	}
	if in.Role != "editor" {
		in.Role = "viewer"
	}
	token := newID() + newID()
	exp := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	_, err := a.db.Exec("INSERT INTO invites(id,family_id,email,token,role,expires_at) VALUES(?,?,?,?,?,?)", newID(), fid, strings.ToLower(strings.TrimSpace(in.Email)), token, in.Role, exp)
	if err != nil {
		errorJSON(w, 500, "database_error", "创建邀请失败")
		return
	}
	a.activity(fid, userID(r.Context()), "invite.created", "family", fid, "创建了成员邀请")
	jsonResponse(w, 201, map[string]any{"token": token, "expires_at": exp})
}
func (a *app) acceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var fid, role, exp, accepted string
	if a.db.QueryRow("SELECT family_id,role,expires_at,accepted_at FROM invites WHERE token=?", token).Scan(&fid, &role, &exp, &accepted) != nil || accepted != "" {
		errorJSON(w, 404, "invite_invalid", "邀请无效")
		return
	}
	if t, _ := time.Parse(time.RFC3339, exp); time.Now().After(t) {
		errorJSON(w, 410, "invite_expired", "邀请已过期")
		return
	}
	var inviteEmail, currentEmail string
	if a.db.QueryRow("SELECT email FROM invites WHERE token=?", token).Scan(&inviteEmail) == nil && a.db.QueryRow("SELECT email FROM users WHERE id=?", userID(r.Context())).Scan(&currentEmail) == nil && !strings.EqualFold(inviteEmail, currentEmail) {
		errorJSON(w, http.StatusForbidden, "invite_email_mismatch", "当前账号邮箱与邀请邮箱不一致")
		return
	}
	if _, err := a.db.Exec("INSERT OR IGNORE INTO family_members(id,family_id,user_id,role,joined_at) VALUES(?,?,?,?,?)", newID(), fid, userID(r.Context()), role, time.Now().UTC().Format(time.RFC3339)); err != nil {
		errorJSON(w, 500, "database_error", "加入家族失败")
		return
	}
	a.db.Exec("UPDATE invites SET accepted_at=? WHERE token=?", time.Now().UTC().Format(time.RFC3339), token)
	a.activity(fid, userID(r.Context()), "member.joined", "family", fid, "加入了家族")
	jsonResponse(w, 200, map[string]any{"family_id": fid, "joined": true})
}

func (a *app) listGenealogies(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT g.id,g.family_id,g.name,g.description,g.created_by,g.created_at,g.updated_at,(SELECT COUNT(*) FROM genealogy_persons gp WHERE gp.genealogy_id=g.id) FROM genealogies g WHERE g.family_id=? ORDER BY g.updated_at DESC`, fid)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取族谱失败")
		return
	}
	defer rows.Close()
	out := []genealogy{}
	for rows.Next() {
		var g genealogy
		if rows.Scan(&g.ID, &g.FamilyID, &g.Name, &g.Description, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt, &g.PersonCount) == nil {
			out = append(out, g)
		}
	}
	jsonResponse(w, 200, out)
}
func (a *app) createGenealogy(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct{ Name, Description string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		errorJSON(w, 400, "invalid_input", "族谱名称不能为空")
		return
	}
	now, id := time.Now().UTC().Format(time.RFC3339), newID()
	_, err := a.db.Exec("INSERT INTO genealogies(id,family_id,name,description,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?)", id, fid, strings.TrimSpace(in.Name), in.Description, userID(r.Context()), now, now)
	if err != nil {
		errorJSON(w, 500, "database_error", "创建族谱失败")
		return
	}
	a.activity(fid, userID(r.Context()), "genealogy.created", "genealogy", id, "创建了族谱")
	a.getGenealogyByID(w, id)
}
func (a *app) getGenealogy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genealogyID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	a.getGenealogyByID(w, id)
}
func (a *app) getGenealogyByID(w http.ResponseWriter, id string) {
	var g genealogy
	err := a.db.QueryRow(`SELECT g.id,g.family_id,g.name,g.description,g.created_by,g.created_at,g.updated_at,(SELECT COUNT(*) FROM genealogy_persons WHERE genealogy_id=g.id) FROM genealogies g WHERE g.id=?`, id).Scan(&g.ID, &g.FamilyID, &g.Name, &g.Description, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt, &g.PersonCount)
	if err != nil {
		errorJSON(w, 404, "not_found", "族谱不存在")
		return
	}
	jsonResponse(w, 200, g)
}
func (a *app) updateGenealogy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genealogyID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct{ Name, Description string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		errorJSON(w, 400, "invalid_input", "请求参数无效")
		return
	}
	_, err := a.db.Exec("UPDATE genealogies SET name=?,description=?,updated_at=? WHERE id=?", strings.TrimSpace(in.Name), strings.TrimSpace(in.Description), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		errorJSON(w, 500, "database_error", "更新族谱失败")
		return
	}
	a.activity(fid, userID(r.Context()), "genealogy.updated", "genealogy", id, "更新了族谱资料")
	a.getGenealogyByID(w, id)
}
func (a *app) deleteGenealogy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genealogyID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM genealogies WHERE id=?", id); err != nil {
		errorJSON(w, 500, "database_error", "删除族谱失败")
		return
	}
	jsonResponse(w, 200, map[string]any{"deleted": true})
}

func (a *app) listGenealogyPersons(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genealogyID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT p.id,p.family_id,p.name,p.gender,p.birth_date,p.death_date,p.birthplace,p.occupation,p.biography,COALESCE(p.claimed_by,''),p.created_at,p.updated_at
		FROM persons p JOIN genealogy_persons gp ON gp.person_id=p.id WHERE gp.genealogy_id=? ORDER BY p.name`, id)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取族谱人物失败")
		return
	}
	defer rows.Close()
	out := []person{}
	for rows.Next() {
		var p person
		if rows.Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.Biography, &p.ClaimedBy, &p.CreatedAt, &p.UpdatedAt) == nil {
			out = append(out, p)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) addGenealogyPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "genealogyID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct {
		PersonID string `json:"person_id"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.PersonID) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择人物")
		return
	}
	if !a.personInFamily(in.PersonID, fid) {
		errorJSON(w, http.StatusBadRequest, "person_invalid", "人物不属于该家族")
		return
	}
	if _, err := a.db.Exec("INSERT INTO genealogy_persons(genealogy_id,person_id) VALUES(?,?)", id, in.PersonID); err != nil {
		errorJSON(w, http.StatusConflict, "association_exists", "人物已经关联到该族谱")
		return
	}
	a.activity(fid, userID(r.Context()), "genealogy.person_added", "genealogy", id, "向族谱添加了人物")
	jsonResponse(w, http.StatusCreated, map[string]any{"genealogy_id": id, "person_id": in.PersonID})
}

func (a *app) removeGenealogyPerson(w http.ResponseWriter, r *http.Request) {
	id, pid := chi.URLParam(r, "genealogyID"), chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM genealogies WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "族谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	result, err := a.db.Exec("DELETE FROM genealogy_persons WHERE genealogy_id=? AND person_id=?", id, pid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移除族谱人物失败")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		errorJSON(w, http.StatusNotFound, "association_not_found", "族谱人物关联不存在")
		return
	}
	a.activity(fid, userID(r.Context()), "genealogy.person_removed", "genealogy", id, "从族谱移除了人物")
	jsonResponse(w, http.StatusOK, map[string]any{"deleted": true})
}

func (a *app) listPersons(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	query := `SELECT id,family_id,name,gender,birth_date,death_date,birthplace,occupation,biography,COALESCE(claimed_by,''),created_at,updated_at FROM persons WHERE (family_id=? OR id IN (SELECT person_id FROM person_families WHERE family_id=?))`
	args := []any{fid, fid}
	if q != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	query += " ORDER BY updated_at DESC"
	rows, err := a.db.Query(query, args...)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取人物失败")
		return
	}
	defer rows.Close()
	out := []person{}
	for rows.Next() {
		var p person
		if rows.Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.Biography, &p.ClaimedBy, &p.CreatedAt, &p.UpdatedAt) == nil {
			out = append(out, p)
		}
	}
	jsonResponse(w, 200, out)
}
func (a *app) createPerson(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct{ Name, Gender, BirthDate, DeathDate, Birthplace, Occupation, Biography, GenealogyID string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		errorJSON(w, 400, "invalid_input", "姓名不能为空")
		return
	}
	if in.Gender != "male" && in.Gender != "female" {
		in.Gender = "unknown"
	}
	now, id := time.Now().UTC().Format(time.RFC3339), newID()
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, 500, "database_error", "创建人物失败")
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO persons(id,family_id,name,gender,birth_date,death_date,birthplace,occupation,biography,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, fid, strings.TrimSpace(in.Name), in.Gender, in.BirthDate, in.DeathDate, in.Birthplace, in.Occupation, in.Biography, now, now); err != nil {
		errorJSON(w, 500, "database_error", "创建人物失败")
		return
	}
	if in.GenealogyID != "" {
		var genealogyFamily string
		if tx.QueryRow("SELECT family_id FROM genealogies WHERE id=?", in.GenealogyID).Scan(&genealogyFamily) != nil || genealogyFamily != fid {
			errorJSON(w, 400, "genealogy_invalid", "族谱不属于该家族")
			return
		}
		if _, err = tx.Exec("INSERT INTO genealogy_persons(genealogy_id,person_id) VALUES(?,?)", in.GenealogyID, id); err != nil {
			errorJSON(w, 400, "genealogy_invalid", "族谱关联失败")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, 500, "database_error", "创建人物失败")
		return
	}
	a.activity(fid, userID(r.Context()), "person.created", "person", id, "新增了人物 "+in.Name)
	a.getPersonByID(w, id)
}
func (a *app) getPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	a.getPersonByID(w, id)
}
func (a *app) getPersonByID(w http.ResponseWriter, id string) {
	var p person
	if a.db.QueryRow("SELECT id,family_id,name,gender,birth_date,death_date,birthplace,occupation,biography,COALESCE(claimed_by,''),created_at,updated_at FROM persons WHERE id=?", id).Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.Biography, &p.ClaimedBy, &p.CreatedAt, &p.UpdatedAt) != nil {
		errorJSON(w, 404, "not_found", "人物不存在")
		return
	}
	jsonResponse(w, 200, p)
}
func (a *app) updatePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var fid, claimed string
	if a.db.QueryRow("SELECT family_id,COALESCE(claimed_by,'') FROM persons WHERE id=?", id).Scan(&fid, &claimed) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	uid := userID(r.Context())
	role := a.memberRole(fid, uid)
	if role == "viewer" && (claimed != uid) {
		errorJSON(w, 403, "forbidden", "没有编辑权限")
		return
	}
	var in struct{ Name, Gender, BirthDate, DeathDate, Birthplace, Occupation, Biography string }
	if !decode(r, &in) {
		errorJSON(w, 400, "invalid_input", "请求参数无效")
		return
	}
	if in.Gender != "male" && in.Gender != "female" {
		in.Gender = "unknown"
	}
	_, err := a.db.Exec("UPDATE persons SET name=?,gender=?,birth_date=?,death_date=?,birthplace=?,occupation=?,biography=?,updated_at=? WHERE id=?", in.Name, in.Gender, in.BirthDate, in.DeathDate, in.Birthplace, in.Occupation, in.Biography, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		errorJSON(w, 500, "database_error", "更新人物失败")
		return
	}
	a.activity(fid, uid, "person.updated", "person", id, "更新了人物资料")
	a.getPersonByID(w, id)
}
func (a *app) deletePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM persons WHERE id=?", id); err != nil {
		errorJSON(w, 500, "database_error", "删除人物失败")
		return
	}
	jsonResponse(w, 200, map[string]any{"deleted": true})
}

func (a *app) listPersonGenealogies(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", pid).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT g.id,g.family_id,g.name,g.description,g.created_by,g.created_at,g.updated_at,
		(SELECT COUNT(*) FROM genealogy_persons gp2 WHERE gp2.genealogy_id=g.id) FROM genealogies g JOIN genealogy_persons gp ON gp.genealogy_id=g.id WHERE gp.person_id=? ORDER BY g.updated_at DESC`, pid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取人物所属族谱失败")
		return
	}
	defer rows.Close()
	out := []genealogy{}
	for rows.Next() {
		var item genealogy
		if rows.Scan(&item.ID, &item.FamilyID, &item.Name, &item.Description, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.PersonCount) == nil {
			out = append(out, item)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) listPersonBooks(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", pid).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query("SELECT id,family_id,title,COALESCE(root_person_id,''),content_json,created_by,created_at,updated_at FROM books WHERE root_person_id=? ORDER BY updated_at DESC", pid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取人物家谱失败")
		return
	}
	defer rows.Close()
	out := []book{}
	for rows.Next() {
		var item book
		if rows.Scan(&item.ID, &item.FamilyID, &item.Title, &item.RootPersonID, &item.ContentJSON, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt) == nil {
			out = append(out, item)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) listPersonPhotos(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", pid).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT p.id,p.family_id,p.person_id,p.path,p.original_name,p.caption,p.category,p.taken_at,p.location,p.created_at
		FROM photos p JOIN photo_persons pp ON pp.photo_id=p.id WHERE pp.person_id=? ORDER BY p.created_at DESC`, pid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取人物照片失败")
		return
	}
	defer rows.Close()
	out := []photo{}
	for rows.Next() {
		var item photo
		if rows.Scan(&item.ID, &item.FamilyID, &item.PersonID, &item.Path, &item.OriginalName, &item.Caption, &item.Category, &item.TakenAt, &item.Location, &item.CreatedAt) == nil {
			item.Path = "/media/" + strings.ReplaceAll(item.Path, "\\", "/")
			out = append(out, item)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

var inverseRelation = map[string]string{"FATHER": "SON", "MOTHER": "SON", "SON": "FATHER", "DAUGHTER": "FATHER", "HUSBAND": "WIFE", "WIFE": "HUSBAND", "BROTHER": "BROTHER", "SISTER": "SISTER", "GRANDFATHER": "GRANDCHILD", "GRANDMOTHER": "GRANDCHILD", "MATERNAL_GRANDFATHER": "GRANDCHILD", "MATERNAL_GRANDMOTHER": "GRANDCHILD", "UNCLE": "NEPHEW_NIECE", "AUNT": "NEPHEW_NIECE", "CUSTOM": "CUSTOM"}

func (a *app) createRelation(w http.ResponseWriter, r *http.Request) {
	from := chi.URLParam(r, "personID")
	var homeFam string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", from).Scan(&homeFam) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	// 关系归属家族：可显式指定（多家族关联场景，关系各家族单独维护）；默认该人物主家族
	var in struct{ ToPersonID, RelationType, CustomName, Note, FamilyID string }
	if !decode(r, &in) || in.ToPersonID == "" {
		errorJSON(w, 400, "invalid_input", "请选择关联人物")
		return
	}
	fid := in.FamilyID
	if fid == "" {
		fid = homeFam
	}
	var inFam int
	if a.db.QueryRow(`SELECT COUNT(*) FROM (SELECT family_id AS x FROM persons WHERE id=? UNION SELECT family_id FROM person_families WHERE person_id=?) WHERE x=?`, from, from, fid).Scan(&inFam) != nil || inFam == 0 {
		errorJSON(w, 400, "family_invalid", "请选择正确的家族")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	if in.RelationType == "CUSTOM" && strings.TrimSpace(in.CustomName) == "" {
		errorJSON(w, 400, "invalid_input", "请输入自定义关系")
		return
	}
	var one int
	if a.db.QueryRow(`SELECT 1 FROM persons WHERE id=? AND (family_id=? OR id IN (SELECT person_id FROM person_families WHERE family_id=?))`, in.ToPersonID, fid, fid).Scan(&one) != nil {
		errorJSON(w, 400, "person_invalid", "关联人物无效")
		return
	}
	forward := newID()
	inv := newID()
	now := time.Now().UTC().Format(time.RFC3339)
	inverse := inverseRelation[in.RelationType]
	if inverse == "" {
		inverse = "CUSTOM"
	}
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, 500, "database_error", "创建关系失败")
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO person_relations(id,family_id,from_person_id,to_person_id,relation_type,custom_name,note,inverse_id,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)", forward, fid, from, in.ToPersonID, in.RelationType, in.CustomName, in.Note, inv, userID(r.Context()), now)
	if err == nil {
		_, err = tx.Exec("INSERT INTO person_relations(id,family_id,from_person_id,to_person_id,relation_type,custom_name,note,inverse_id,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)", inv, fid, in.ToPersonID, from, inverse, in.CustomName, in.Note, forward, userID(r.Context()), now)
	}
	if err != nil {
		errorJSON(w, 409, "relation_exists", "关系已存在或创建失败")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, 500, "database_error", "创建关系失败")
		return
	}
	a.activity(fid, userID(r.Context()), "relation.created", "relation", forward, "建立了人物关系")
	jsonResponse(w, 201, map[string]any{"id": forward, "inverse_id": inv})
}
func (a *app) deleteRelation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "relationID")
	var fid, inv string
	if a.db.QueryRow("SELECT family_id,inverse_id FROM person_relations WHERE id=?", id).Scan(&fid, &inv) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "关系不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM person_relations WHERE id=? OR id=?", id, inv); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "删除关系失败")
		return
	}
	a.activity(fid, userID(r.Context()), "relation.deleted", "relation", id, "删除了人物关系")
	jsonResponse(w, 200, map[string]any{"deleted": true})
}

func (a *app) updateRelation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "relationID")
	var fid, inv string
	if a.db.QueryRow("SELECT family_id,inverse_id FROM person_relations WHERE id=?", id).Scan(&fid, &inv) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "关系不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct {
		FromPersonID string `json:"from_person_id"`
		ToPersonID   string `json:"to_person_id"`
		RelationType string `json:"relation_type"`
		CustomName   string `json:"custom_name"`
		Note         string `json:"note"`
	}
	if !decode(r, &in) {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请求参数无效")
		return
	}
	in.RelationType = strings.ToUpper(strings.TrimSpace(in.RelationType))
	if _, ok := inverseRelation[in.RelationType]; !ok && in.RelationType != "CUSTOM" && in.RelationType != "GRANDCHILD" && in.RelationType != "NEPHEW_NIECE" {
		errorJSON(w, http.StatusBadRequest, "invalid_relation", "关系类型无效")
		return
	}
	if in.RelationType == "CUSTOM" && strings.TrimSpace(in.CustomName) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "自定义关系需要填写关系名称")
		return
	}
	from, to := in.FromPersonID, in.ToPersonID
	if from == "" || to == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "缺少关联人物")
		return
	}
	if from == to {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "不能将人物与其自身关联")
		return
	}
	for _, pid := range []string{from, to} {
		if !a.personInFamily(pid, fid) {
			errorJSON(w, http.StatusBadRequest, "person_invalid", "关联人物不属于该家族")
			return
		}
	}
	inverseType := inverseRelation[in.RelationType]
	if inverseType == "" {
		inverseType = "CUSTOM"
	}
	customName := strings.TrimSpace(in.CustomName)
	note := strings.TrimSpace(in.Note)
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "更新关系失败")
		return
	}
	defer tx.Rollback()
	// 若目标方向恰好等于反向行（语义等价，仅换了一种表述方向），
	// 则两端点/类型保持不变，只同步备注与自定义名称。
	sameAsInverse := 0
	if inv != "" {
		_ = tx.QueryRow("SELECT COUNT(*) FROM person_relations WHERE id=? AND from_person_id=? AND to_person_id=? AND relation_type=? AND custom_name=?", inv, from, to, in.RelationType, customName).Scan(&sameAsInverse)
	}
	if inv != "" && sameAsInverse == 1 {
		_, err = tx.Exec("UPDATE person_relations SET custom_name=?,note=? WHERE id=?", customName, note, id)
		if err == nil {
			_, err = tx.Exec("UPDATE person_relations SET custom_name=?,note=? WHERE id=?", customName, note, inv)
		}
	} else {
		_, err = tx.Exec("UPDATE person_relations SET from_person_id=?,to_person_id=?,relation_type=?,custom_name=?,note=? WHERE id=?", from, to, in.RelationType, customName, note, id)
		if err == nil && inv != "" {
			_, err = tx.Exec("UPDATE person_relations SET from_person_id=?,to_person_id=?,relation_type=?,custom_name=?,note=? WHERE id=?", to, from, inverseType, customName, note, inv)
		}
	}
	if err != nil {
		errorJSON(w, http.StatusConflict, "relation_exists", "关系已存在或更新失败")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "更新关系失败")
		return
	}
	a.activity(fid, userID(r.Context()), "relation.updated", "relation", id, "更新了人物关系")
	jsonResponse(w, http.StatusOK, map[string]any{"id": id, "from_person_id": from, "to_person_id": to, "relation_type": in.RelationType, "custom_name": customName, "note": note})
}
func (a *app) personGraph(w http.ResponseWriter, r *http.Request) {
	center := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", center).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT r.id,r.from_person_id,r.to_person_id,r.relation_type,r.custom_name,r.note FROM person_relations r WHERE r.from_person_id=? OR r.to_person_id=?`, center, center)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取关系图失败")
		return
	}
	defer rows.Close()
	edges := []relation{}
	ids := map[string]bool{center: true}
	for rows.Next() {
		var x relation
		if rows.Scan(&x.ID, &x.FromPersonID, &x.ToPersonID, &x.RelationType, &x.CustomName, &x.Note) == nil {
			edges = append(edges, x)
			ids[x.FromPersonID] = true
			ids[x.ToPersonID] = true
		}
	}
	nodes := []person{}
	for id := range ids {
		var p person
		if a.db.QueryRow("SELECT id,family_id,name,gender,birth_date,death_date,birthplace,occupation,biography,COALESCE(claimed_by,''),created_at,updated_at FROM persons WHERE id=?", id).Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.Biography, &p.ClaimedBy, &p.CreatedAt, &p.UpdatedAt) == nil {
			nodes = append(nodes, p)
		}
	}
	jsonResponse(w, 200, map[string]any{"center": center, "nodes": nodes, "edges": edges})
}

func (a *app) familyGraph(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	center := strings.TrimSpace(r.URL.Query().Get("center_person_id"))
	if center != "" && !a.personInFamily(center, fid) {
		errorJSON(w, http.StatusBadRequest, "person_invalid", "中心人物不属于该家族")
		return
	}
	rows, err := a.db.Query(`SELECT id,from_person_id,to_person_id,relation_type,custom_name,note FROM person_relations WHERE family_id=? ORDER BY created_at,id`, fid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取家族关系图失败")
		return
	}
	defer rows.Close()
	edges := []relation{}
	for rows.Next() {
		var item relation
		if rows.Scan(&item.ID, &item.FromPersonID, &item.ToPersonID, &item.RelationType, &item.CustomName, &item.Note) == nil {
			edges = append(edges, item)
		}
	}
	personRows, err := a.db.Query(`SELECT id,family_id,name,gender,birth_date,death_date,birthplace,occupation,biography,COALESCE(claimed_by,''),created_at,updated_at FROM persons WHERE (family_id=? OR id IN (SELECT person_id FROM person_families WHERE family_id=?)) ORDER BY name,id`, fid, fid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取家族人物失败")
		return
	}
	defer personRows.Close()
	nodes := []person{}
	genderByID := map[string]string{}
	for personRows.Next() {
		var item person
		if personRows.Scan(&item.ID, &item.FamilyID, &item.Name, &item.Gender, &item.BirthDate, &item.DeathDate, &item.Birthplace, &item.Occupation, &item.Biography, &item.ClaimedBy, &item.CreatedAt, &item.UpdatedAt) == nil {
			nodes = append(nodes, item)
			genderByID[item.ID] = item.Gender
		}
	}
	// 关系图中的祖辈关系按已保存的父母、子女和配偶关系计算，不写回基础关系表。
	knownPairs := map[string]bool{}
	parentOf := map[string]map[string]bool{}
	spouses := map[string]map[string]bool{}
	addPair := func(from, to string) { knownPairs[from+":"+to] = true; knownPairs[to+":"+from] = true }
	for _, edge := range edges {
		addPair(edge.FromPersonID, edge.ToPersonID)
		switch edge.RelationType {
		case "FATHER", "MOTHER":
			if parentOf[edge.ToPersonID] == nil {
				parentOf[edge.ToPersonID] = map[string]bool{}
			}
			parentOf[edge.ToPersonID][edge.FromPersonID] = true
		case "SON", "DAUGHTER":
			if parentOf[edge.FromPersonID] == nil {
				parentOf[edge.FromPersonID] = map[string]bool{}
			}
			parentOf[edge.FromPersonID][edge.ToPersonID] = true
		case "HUSBAND", "WIFE":
			if spouses[edge.FromPersonID] == nil {
				spouses[edge.FromPersonID] = map[string]bool{}
			}
			spouses[edge.FromPersonID][edge.ToPersonID] = true
		}
	}
	// 深代推导：自每位祖辈沿子辈链条下探，最多上下 18 代。
	// 称谓规则：全由儿子接续 → 祖（孙子/孙女/曾孙…）；经过女儿 → 外（外孙/外曾孙…）。
	const maxDepth = 18
	derivedSeen := map[string]bool{}
	addDerived := func(realGP, realGC string, depth int, maternal bool) {
		if realGP == realGC || depth < 2 || depth > maxDepth {
			return
		}
		if knownPairs[realGP+":"+realGC] || derivedSeen[realGP+"|"+realGC] {
			return
		}
		derivedSeen[realGP+"|"+realGC] = true
		relationType := ""
		if depth == 2 {
			if maternal {
				if genderByID[realGP] == "female" {
					relationType = "MATERNAL_GRANDMOTHER"
				} else {
					relationType = "MATERNAL_GRANDFATHER"
				}
			} else if genderByID[realGP] == "female" {
				relationType = "GRANDMOTHER"
			} else {
				relationType = "GRANDFATHER"
			}
		} else if maternal {
			relationType = fmt.Sprintf("M%d", depth)
		} else {
			relationType = fmt.Sprintf("G%d", depth)
		}
		inverse := "GRANDCHILD"
		id := "derived:" + realGP + ":" + realGC + ":" + relationType
		edges = append(edges,
			// 与库中关系约定一致：边起点为晚辈、终点为长辈时使用祖辈称谓。
			relation{ID: id, FromPersonID: realGC, ToPersonID: realGP, RelationType: relationType},
			relation{ID: id + ":inverse", FromPersonID: realGP, ToPersonID: realGC, RelationType: inverse},
		)
		knownPairs[realGP+":"+realGC] = true
	}
	// 以 start 为祖辈，向下 BFS 收集所有后辈（含代际距离与是否经过女儿）
	type dInfo struct {
		d        int
		maternal bool
	}
	descendants := func(start string) map[string]dInfo {
		out := map[string]dInfo{}
		visited := map[string]bool{start: true}
		type node struct {
			id       string
			d        int
			maternal bool
		}
		queue := []node{}
		for child := range parentOf[start] {
			queue = append(queue, node{child, 1, genderByID[child] == "female"})
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if _, ok := out[cur.id]; !ok {
				out[cur.id] = dInfo{cur.d, cur.maternal}
			}
			if cur.d >= maxDepth {
				continue
			}
			for child := range parentOf[cur.id] {
				if visited[child] {
					continue
				}
				visited[child] = true
				// 是否外系：中间传接辈（不含起点祖辈与终点后代自身）中出现女儿
				m := cur.maternal || (cur.id != start && genderByID[cur.id] == "female")
				queue = append(queue, node{child, cur.d + 1, m})
			}
		}
		return out
	}
	for personID := range parentOf {
		own := descendants(personID)
		for id, info := range own {
			addDerived(personID, id, info.d, info.maternal)
		}
		for spouse := range spouses[personID] {
			bySpouse := descendants(spouse)
			for id, info := range bySpouse {
				addDerived(spouse, id, info.d, info.maternal)
			}
			// 配偶未记录子辈边时，沿用本家后辈（夫妻共享后代）
			if len(bySpouse) == 0 {
				for id, info := range own {
					addDerived(spouse, id, info.d, info.maternal)
				}
			}
		}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"center": center, "nodes": nodes, "edges": edges})
}

func (a *app) listBooks(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query("SELECT id,family_id,title,COALESCE(root_person_id,''),content_json,created_by,created_at,updated_at FROM books WHERE family_id=? ORDER BY updated_at DESC", fid)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取家谱失败")
		return
	}
	defer rows.Close()
	out := []book{}
	for rows.Next() {
		var b book
		if rows.Scan(&b.ID, &b.FamilyID, &b.Title, &b.RootPersonID, &b.ContentJSON, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt) == nil {
			out = append(out, b)
		}
	}
	jsonResponse(w, 200, out)
}
func (a *app) createBook(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct {
		Title, RootPersonID string
		ContentJSON         json.RawMessage
	}
	if !decode(r, &in) || strings.TrimSpace(in.Title) == "" {
		errorJSON(w, 400, "invalid_input", "家谱标题不能为空")
		return
	}
	content := string(in.ContentJSON)
	if content == "" || content == "null" {
		content = `{"type":"doc","content":[{"type":"paragraph"}]}`
	}
	now, id := time.Now().UTC().Format(time.RFC3339), newID()
	var rootPersonID any
	if in.RootPersonID != "" {
		if !a.personInFamily(in.RootPersonID, fid) {
			errorJSON(w, http.StatusBadRequest, "person_invalid", "关联人物不属于该家族")
			return
		}
		rootPersonID = in.RootPersonID
	}
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "创建家谱失败")
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO books(id,family_id,title,root_person_id,content_json,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", id, fid, strings.TrimSpace(in.Title), rootPersonID, content, userID(r.Context()), now, now)
	if err == nil {
		_, err = tx.Exec("INSERT INTO book_collaborators(book_id,user_id) VALUES(?,?)", id, userID(r.Context()))
	}
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "创建家谱失败")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "创建家谱失败")
		return
	}
	a.activity(fid, userID(r.Context()), "book.created", "book", id, "创建了家谱")
	a.getBookByID(w, id)
}
func (a *app) getBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "bookID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	a.getBookByID(w, id)
}
func (a *app) getBookByID(w http.ResponseWriter, id string) {
	var b book
	if a.db.QueryRow("SELECT id,family_id,title,COALESCE(root_person_id,''),content_json,created_by,created_at,updated_at FROM books WHERE id=?", id).Scan(&b.ID, &b.FamilyID, &b.Title, &b.RootPersonID, &b.ContentJSON, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt) != nil {
		errorJSON(w, 404, "not_found", "家谱不存在")
		return
	}
	jsonResponse(w, 200, b)
}
func (a *app) updateBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "bookID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureBookEditor(w, r, id, fid) {
		return
	}
	var in struct {
		Title, RootPersonID string
		ContentJSON         json.RawMessage
	}
	if !decode(r, &in) {
		errorJSON(w, 400, "invalid_input", "请求参数无效")
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "家谱标题不能为空")
		return
	}
	content := string(in.ContentJSON)
	if content == "" || content == "null" {
		content = `{"type":"doc","content":[{"type":"paragraph"}]}`
	}
	var rootPersonID any
	if in.RootPersonID != "" {
		if !a.personInFamily(in.RootPersonID, fid) {
			errorJSON(w, http.StatusBadRequest, "person_invalid", "关联人物不属于该家族")
			return
		}
		rootPersonID = in.RootPersonID
	}
	_, err := a.db.Exec("UPDATE books SET title=?,root_person_id=?,content_json=?,updated_at=? WHERE id=?", strings.TrimSpace(in.Title), rootPersonID, content, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		errorJSON(w, 500, "database_error", "更新家谱失败")
		return
	}
	a.activity(fid, userID(r.Context()), "book.updated", "book", id, "更新了家谱")
	a.getBookByID(w, id)
}
func (a *app) deleteBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "bookID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM books WHERE id=?", id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "删除家谱失败")
		return
	}
	a.activity(fid, userID(r.Context()), "book.deleted", "book", id, "删除了家谱")
	jsonResponse(w, 200, map[string]any{"deleted": true})
}

func (a *app) listBookCollaborators(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "bookID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT u.id,u.email,u.display_name,u.avatar_url,u.created_at FROM book_collaborators bc JOIN users u ON u.id=bc.user_id WHERE bc.book_id=? ORDER BY u.display_name`, id)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取家谱协作者失败")
		return
	}
	defer rows.Close()
	out := []user{}
	for rows.Next() {
		var item user
		if rows.Scan(&item.ID, &item.Email, &item.DisplayName, &item.AvatarURL, &item.CreatedAt) == nil {
			out = append(out, item)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) addBookCollaborator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "bookID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	var in struct {
		UserID string `json:"user_id"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.UserID) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择协作者")
		return
	}
	if a.memberRole(fid, in.UserID) == "" {
		errorJSON(w, http.StatusBadRequest, "member_invalid", "协作者不是该家族成员")
		return
	}
	if _, err := a.db.Exec("INSERT INTO book_collaborators(book_id,user_id) VALUES(?,?)", id, in.UserID); err != nil {
		errorJSON(w, http.StatusConflict, "collaborator_exists", "该成员已经是协作者")
		return
	}
	a.activity(fid, userID(r.Context()), "book.collaborator_added", "book", id, "添加了家谱协作者")
	jsonResponse(w, http.StatusCreated, map[string]any{"book_id": id, "user_id": in.UserID})
}

func (a *app) removeBookCollaborator(w http.ResponseWriter, r *http.Request) {
	id, uid := chi.URLParam(r, "bookID"), chi.URLParam(r, "userID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM books WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "家谱不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "owner") {
		return
	}
	result, err := a.db.Exec("DELETE FROM book_collaborators WHERE book_id=? AND user_id=?", id, uid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移除家谱协作者失败")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		errorJSON(w, http.StatusNotFound, "collaborator_not_found", "协作者不存在")
		return
	}
	a.activity(fid, userID(r.Context()), "book.collaborator_removed", "book", id, "移除了家谱协作者")
	jsonResponse(w, http.StatusOK, map[string]any{"deleted": true})
}

func (a *app) uploadPhoto(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", pid).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		errorJSON(w, 400, "file_invalid", "照片不能超过 12MB")
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		errorJSON(w, 400, "file_required", "请选择照片")
		return
	}
	defer f.Close()
	if !isImage(header) {
		errorJSON(w, 400, "file_invalid", "仅支持 jpg、png、webp 图片")
		return
	}
	id := newID()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".jpg"
	}
	rel := filepath.Join(fid, pid, id+ext)
	path := filepath.Join(a.storageDir, rel)
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		errorJSON(w, 500, "storage_error", "照片保存失败")
		return
	}
	dst, err := os.Create(path)
	if err != nil {
		errorJSON(w, 500, "storage_error", "照片保存失败")
		return
	}
	_, copyErr := io.Copy(dst, f)
	dst.Close()
	if copyErr != nil {
		errorJSON(w, 500, "storage_error", "照片保存失败")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	caption := r.FormValue("caption")
	category := r.FormValue("category")
	if category == "" {
		category = "other"
	}
	_, err = a.db.Exec("INSERT INTO photos(id,family_id,person_id,path,original_name,caption,category,taken_at,location,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", id, fid, pid, rel, header.Filename, caption, category, r.FormValue("taken_at"), r.FormValue("location"), userID(r.Context()), now)
	if err != nil {
		_ = os.Remove(path)
		errorJSON(w, 500, "database_error", "照片记录失败")
		return
	}
	if _, err = a.db.Exec("INSERT INTO photo_persons(photo_id,person_id) VALUES(?,?)", id, pid); err != nil {
		_ = os.Remove(path)
		_, _ = a.db.Exec("DELETE FROM photos WHERE id=?", id)
		errorJSON(w, http.StatusInternalServerError, "database_error", "照片人物关联失败")
		return
	}
	a.activity(fid, userID(r.Context()), "photo.created", "photo", id, "上传了一张照片")
	a.getPhotoByIDStatus(w, id, http.StatusCreated)
}

func (a *app) listPhotos(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query("SELECT id,family_id,person_id,path,original_name,caption,category,taken_at,location,created_at FROM photos WHERE family_id=? ORDER BY created_at DESC", fid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取照片失败")
		return
	}
	defer rows.Close()
	out := []photo{}
	for rows.Next() {
		var item photo
		if rows.Scan(&item.ID, &item.FamilyID, &item.PersonID, &item.Path, &item.OriginalName, &item.Caption, &item.Category, &item.TakenAt, &item.Location, &item.CreatedAt) == nil {
			item.Path = "/media/" + strings.ReplaceAll(item.Path, "\\", "/")
			out = append(out, item)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) getPhoto(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "photoID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM photos WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	a.getPhotoByID(w, id)
}

func (a *app) getPhotoByID(w http.ResponseWriter, id string) {
	a.getPhotoByIDStatus(w, id, http.StatusOK)
}

func (a *app) getPhotoByIDStatus(w http.ResponseWriter, id string, status int) {
	var item photo
	if a.db.QueryRow("SELECT id,family_id,person_id,path,original_name,caption,category,taken_at,location,created_at FROM photos WHERE id=?", id).Scan(&item.ID, &item.FamilyID, &item.PersonID, &item.Path, &item.OriginalName, &item.Caption, &item.Category, &item.TakenAt, &item.Location, &item.CreatedAt) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	item.Path = "/media/" + strings.ReplaceAll(item.Path, "\\", "/")
	jsonResponse(w, status, item)
}

func (a *app) updatePhoto(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "photoID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM photos WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct {
		Caption  *string `json:"caption"`
		Category *string `json:"category"`
		TakenAt  *string `json:"taken_at"`
		Location *string `json:"location"`
	}
	if !decode(r, &in) {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请求参数无效")
		return
	}
	var current photo
	if a.db.QueryRow("SELECT id,family_id,person_id,path,original_name,caption,category,taken_at,location,created_at FROM photos WHERE id=?", id).Scan(&current.ID, &current.FamilyID, &current.PersonID, &current.Path, &current.OriginalName, &current.Caption, &current.Category, &current.TakenAt, &current.Location, &current.CreatedAt) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if in.Caption != nil {
		current.Caption = strings.TrimSpace(*in.Caption)
	}
	if in.Category != nil {
		current.Category = strings.TrimSpace(*in.Category)
	}
	if in.TakenAt != nil {
		current.TakenAt = strings.TrimSpace(*in.TakenAt)
	}
	if in.Location != nil {
		current.Location = strings.TrimSpace(*in.Location)
	}
	if current.Category == "" {
		current.Category = "other"
	}
	if _, err := a.db.Exec("UPDATE photos SET caption=?,category=?,taken_at=?,location=? WHERE id=?", current.Caption, current.Category, current.TakenAt, current.Location, id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "更新照片失败")
		return
	}
	a.activity(fid, userID(r.Context()), "photo.updated", "photo", id, "更新了照片资料")
	a.getPhotoByID(w, id)
}

func (a *app) listPhotoPersons(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "photoID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM photos WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query(`SELECT p.id,p.family_id,p.name,p.gender,p.birth_date,p.death_date,p.birthplace,p.occupation,p.biography,COALESCE(p.claimed_by,''),p.created_at,p.updated_at
		FROM persons p JOIN photo_persons pp ON pp.person_id=p.id WHERE pp.photo_id=? ORDER BY p.name`, id)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取照片人物失败")
		return
	}
	defer rows.Close()
	out := []person{}
	for rows.Next() {
		var p person
		if rows.Scan(&p.ID, &p.FamilyID, &p.Name, &p.Gender, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Occupation, &p.Biography, &p.ClaimedBy, &p.CreatedAt, &p.UpdatedAt) == nil {
			out = append(out, p)
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) addPhotoPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "photoID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM photos WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var in struct {
		PersonID string `json:"person_id"`
	}
	if !decode(r, &in) || strings.TrimSpace(in.PersonID) == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择人物")
		return
	}
	if !a.personInFamily(in.PersonID, fid) {
		errorJSON(w, http.StatusBadRequest, "person_invalid", "人物不属于该家族")
		return
	}
	if _, err := a.db.Exec("INSERT INTO photo_persons(photo_id,person_id) VALUES(?,?)", id, in.PersonID); err != nil {
		errorJSON(w, http.StatusConflict, "association_exists", "照片已经关联该人物")
		return
	}
	a.activity(fid, userID(r.Context()), "photo.person_added", "photo", id, "为照片关联了人物")
	jsonResponse(w, http.StatusCreated, map[string]any{"photo_id": id, "person_id": in.PersonID})
}

func (a *app) removePhotoPerson(w http.ResponseWriter, r *http.Request) {
	id, pid := chi.URLParam(r, "photoID"), chi.URLParam(r, "personID")
	var fid string
	if a.db.QueryRow("SELECT family_id FROM photos WHERE id=?", id).Scan(&fid) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	var count int
	if a.db.QueryRow("SELECT COUNT(*) FROM photo_persons WHERE photo_id=?", id).Scan(&count) == nil && count <= 1 {
		errorJSON(w, http.StatusBadRequest, "association_required", "照片至少需要关联一个人物")
		return
	}
	result, err := a.db.Exec("DELETE FROM photo_persons WHERE photo_id=? AND person_id=?", id, pid)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移除照片人物失败")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		errorJSON(w, http.StatusNotFound, "association_not_found", "照片人物关联不存在")
		return
	}
	a.activity(fid, userID(r.Context()), "photo.person_removed", "photo", id, "移除了照片人物关联")
	jsonResponse(w, http.StatusOK, map[string]any{"deleted": true})
}

func (a *app) deletePhoto(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "photoID")
	var fid, path string
	if a.db.QueryRow("SELECT family_id,path FROM photos WHERE id=?", id).Scan(&fid, &path) != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "照片不存在")
		return
	}
	if !a.ensureRole(w, r, fid, "editor") {
		return
	}
	if _, err := a.db.Exec("DELETE FROM photos WHERE id=?", id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "删除照片失败")
		return
	}
	_ = os.Remove(filepath.Join(a.storageDir, path))
	a.activity(fid, userID(r.Context()), "photo.deleted", "photo", id, "删除了照片")
	jsonResponse(w, 200, map[string]any{"deleted": true})
}
// ---- 人物多家族关联（人物关系各家族单独维护，不共用） ----
func (a *app) personFamilies(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var homeFam string
	if err := a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&homeFam); err != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if a.memberRole(homeFam, userID(r.Context())) == "" {
		errorJSON(w, http.StatusForbidden, "forbidden", "不是该家族成员")
		return
	}
	rows, err := a.db.Query(`SELECT f.id,f.name,f.surname FROM families f WHERE f.id=? OR f.id IN (SELECT family_id FROM person_families WHERE person_id=?) ORDER BY f.created_at, f.id`, homeFam, id)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "读取家族失败")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var fid, name, surname string
		if rows.Scan(&fid, &name, &surname) == nil {
			out = append(out, map[string]any{"id": fid, "name": name, "surname": surname, "home": fid == homeFam})
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

func (a *app) linkPersonFamily(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var homeFam string
	if err := a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&homeFam); err != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureRole(w, r, homeFam, "editor") {
		return
	}
	var in struct{ FamilyID string `json:"family_id"` }
	if !decode(r, &in) || in.FamilyID == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择家族")
		return
	}
	if in.FamilyID == homeFam {
		errorJSON(w, http.StatusBadRequest, "same_family", "该人物已经在自己的主家族中")
		return
	}
	role := a.memberRole(in.FamilyID, userID(r.Context()))
	if role == "" {
		errorJSON(w, http.StatusForbidden, "forbidden", "你不是目标家族的成员")
		return
	}
	if role == "viewer" {
		errorJSON(w, http.StatusForbidden, "forbidden", "关联人物需要目标家族的编辑权限")
		return
	}
	if _, err := a.db.Exec("INSERT INTO person_families(person_id,family_id) VALUES(?,?)", id, in.FamilyID); err != nil {
		errorJSON(w, http.StatusConflict, "linked", "该人物已关联此家族")
		return
	}
	a.activity(in.FamilyID, userID(r.Context()), "person.linked", "person", id, "将人物关联到本家族")
	jsonResponse(w, http.StatusCreated, map[string]any{"linked": true, "person_id": id, "family_id": in.FamilyID})
}

func (a *app) unlinkPersonFamily(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	familyID := chi.URLParam(r, "familyID")
	var homeFam string
	if err := a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&homeFam); err != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if familyID == homeFam {
		errorJSON(w, http.StatusBadRequest, "home_family", "不能移除人物的主家族")
		return
	}
	if !a.ensureRole(w, r, familyID, "editor") {
		return
	}
	result, err := a.db.Exec("DELETE FROM person_families WHERE person_id=? AND family_id=?", id, familyID)
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "取消关联失败")
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		errorJSON(w, http.StatusNotFound, "not_linked", "该人物未关联此家族")
		return
	}
	a.activity(familyID, userID(r.Context()), "person.unlinked", "person", id, "将人物移出本家族关联")
	jsonResponse(w, http.StatusOK, map[string]any{"removed": true})
}

func (a *app) movePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "personID")
	var fromFam string
	if err := a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", id).Scan(&fromFam); err != nil {
		errorJSON(w, http.StatusNotFound, "not_found", "人物不存在")
		return
	}
	if !a.ensureRole(w, r, fromFam, "editor") {
		return
	}
	var in struct{ FamilyID string `json:"family_id"` }
	if !decode(r, &in) || in.FamilyID == "" {
		errorJSON(w, http.StatusBadRequest, "invalid_input", "请选择目标家族")
		return
	}
	if in.FamilyID == fromFam {
		errorJSON(w, http.StatusBadRequest, "same_family", "人物已经在目标家族中")
		return
	}
	role := a.memberRole(in.FamilyID, userID(r.Context()))
	if role == "" {
		errorJSON(w, http.StatusForbidden, "forbidden", "你不是目标家族的成员")
		return
	}
	if role == "viewer" {
		errorJSON(w, http.StatusForbidden, "forbidden", "移动人物需要目标家族的编辑权限")
		return
	}
	// 检查：与仍留在本家族的人物是否存在关系（避免产生跨家族的悬空关系）
	var leftovers []string
	rows, err := a.db.Query(`SELECT p.name FROM person_relations r JOIN persons p ON p.id=r.from_person_id WHERE r.family_id=? AND r.to_person_id=? AND p.id<>? UNION SELECT p.name FROM person_relations r JOIN persons p ON p.id=r.to_person_id WHERE r.family_id=? AND r.from_person_id=? AND p.id<>?`, fromFam, id, id, fromFam, id, id)
	if err == nil {
		defer rows.Close()
		seen := map[string]bool{}
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil && !seen[name] {
				seen[name] = true
				leftovers = append(leftovers, name)
			}
		}
	}
	if len(leftovers) > 0 {
		errorJSON(w, http.StatusBadRequest, "relations_remain", "该人物与仍在这一分支的人物存在关系："+strings.Join(leftovers, "、")+"。请先处理这些关系（或把整个分支一起移入新家族）")
		return
	}
	uid := userID(r.Context())
	tx, err := a.db.Begin()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移动人物失败")
		return
	}
	defer tx.Rollback()
	// 关系行迁移（正反两行都引用该人物）
	if _, err = tx.Exec("UPDATE person_relations SET family_id=? WHERE family_id=? AND (from_person_id=? OR to_person_id=?)", in.FamilyID, fromFam, id, id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "迁移关系失败")
		return
	}
	// 人物归属
	if _, err = tx.Exec("UPDATE persons SET family_id=? WHERE id=?", in.FamilyID, id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移动人物失败")
		return
	}
	// 跨家族引用清理
	if _, err = tx.Exec("DELETE FROM genealogy_persons WHERE person_id=?", id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "清理族谱关联失败")
		return
	}
	if _, err = tx.Exec("DELETE FROM photo_persons WHERE person_id=?", id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "清理照片关联失败")
		return
	}
	if _, err = tx.Exec("UPDATE books SET root_person_id=NULL WHERE root_person_id=?", id); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "清理家谱关联失败")
		return
	}
	if err = tx.Commit(); err != nil {
		errorJSON(w, http.StatusInternalServerError, "database_error", "移动人物失败")
		return
	}
	a.activity(fromFam, uid, "person.moved_out", "person", id, "将人物移出本家族")
	a.activity(in.FamilyID, uid, "person.moved_in", "person", id, "将人物移入本家族")
	jsonResponse(w, http.StatusOK, map[string]any{"moved": true, "person_id": id, "family_id": in.FamilyID})
}

func (a *app) claimPerson(w http.ResponseWriter, r *http.Request) {	id := chi.URLParam(r, "personID")
	var fid, claimed string
	if a.db.QueryRow("SELECT family_id,COALESCE(claimed_by,'') FROM persons WHERE id=?", id).Scan(&fid, &claimed) != nil || !a.ensureMember(w, r, fid) {
		return
	}
	if claimed != "" {
		errorJSON(w, 409, "already_claimed", "人物已经被认领")
		return
	}
	result, err := a.db.Exec("UPDATE persons SET claimed_by=?,updated_at=? WHERE id=? AND claimed_by IS NULL", userID(r.Context()), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		errorJSON(w, 500, "database_error", "认领失败")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		errorJSON(w, http.StatusConflict, "already_claimed", "人物已经被认领")
		return
	}
	a.activity(fid, userID(r.Context()), "person.claimed", "person", id, "认领了人物")
	jsonResponse(w, 200, map[string]any{"claimed": true})
}

func (a *app) activities(w http.ResponseWriter, r *http.Request) {
	fid := chi.URLParam(r, "familyID")
	if !a.ensureMember(w, r, fid) {
		return
	}
	rows, err := a.db.Query("SELECT id,actor_id,type,target_type,target_id,summary,created_at FROM activities WHERE family_id=? ORDER BY created_at DESC LIMIT 30", fid)
	if err != nil {
		errorJSON(w, 500, "database_error", "读取动态失败")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, actor, t, tt, tid, s, created string
		if rows.Scan(&id, &actor, &t, &tt, &tid, &s, &created) == nil {
			out = append(out, map[string]any{"id": id, "actor_id": actor, "type": t, "target_type": tt, "target_id": tid, "summary": s, "created_at": created})
		}
	}
	jsonResponse(w, 200, out)
}

func (a *app) findUser(id string) (user, error) {
	var u user
	err := a.db.QueryRow("SELECT id,email,display_name,avatar_url,created_at FROM users WHERE id=?", id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL, &u.CreatedAt)
	return u, err
}
func (a *app) ensureMember(w http.ResponseWriter, r *http.Request, fid string) bool {
	if a.memberRole(fid, userID(r.Context())) == "" {
		errorJSON(w, 403, "forbidden", "不是该家族成员")
		return false
	}
	return true
}
func (a *app) ensureRole(w http.ResponseWriter, r *http.Request, fid, required string) bool {
	role := a.memberRole(fid, userID(r.Context()))
	allowed := role == "owner" || (required == "editor" && role == "editor")
	if !allowed {
		errorJSON(w, 403, "forbidden", "没有执行该操作的权限")
		return false
	}
	return true
}
func (a *app) memberRole(fid, uid string) string {
	var role string
	_ = a.db.QueryRow("SELECT role FROM family_members WHERE family_id=? AND user_id=? AND status='active'", fid, uid).Scan(&role)
	return role
}

// 人物是否属于某家族：主家族或已被关联（person_families）
func (a *app) personInFamily(pid, fid string) bool {
	var home string
	if a.db.QueryRow("SELECT family_id FROM persons WHERE id=?", pid).Scan(&home) != nil {
		return false
	}
	if home == fid {
		return true
	}
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM person_families WHERE person_id=? AND family_id=?", pid, fid).Scan(&n)
	return n > 0
}

func (a *app) ensureBookEditor(w http.ResponseWriter, r *http.Request, bookID, fid string) bool {
	role := a.memberRole(fid, userID(r.Context()))
	if role == "owner" || role == "editor" {
		return true
	}
	var exists int
	if a.db.QueryRow("SELECT 1 FROM book_collaborators WHERE book_id=? AND user_id=?", bookID, userID(r.Context())).Scan(&exists) == nil {
		return true
	}
	errorJSON(w, http.StatusForbidden, "forbidden", "没有编辑该家谱的权限")
	return false
}
func (a *app) activity(fid, actor, t, tt, tid, summary string) {
	_, _ = a.db.Exec("INSERT INTO activities(id,family_id,actor_id,type,target_type,target_id,summary,created_at) VALUES(?,?,?,?,?,?,?,?)", newID(), fid, actor, t, tt, tid, summary, time.Now().UTC().Format(time.RFC3339))
}

func userID(ctx context.Context) string { v, _ := ctx.Value(userIDKey).(string); return v }
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func decode(r *http.Request, v any) bool { return json.NewDecoder(r.Body).Decode(v) == nil }
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v, "meta": map[string]any{}})
}
func errorJSON(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": message, "details": map[string]any{}}})
}
func isImage(h *multipart.FileHeader) bool {
	ext := strings.ToLower(filepath.Ext(h.Filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp"
}

var _ = fmt.Sprintf
var _ = strconv.Itoa
