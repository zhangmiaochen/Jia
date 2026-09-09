package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := os.Getenv("JIA_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "jia.db")
	}
	if abs, err := filepath.Abs(dbPath); err == nil {
		dbPath = abs
	}
	fmt.Printf("正在迁移数据库: %s\n", dbPath)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		panic(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	sqlBytes, err := os.ReadFile(filepath.Join("migrations", "001_init.sql"))
	if err != nil {
		panic(err)
	}
	if _, err = db.Exec(string(sqlBytes)); err != nil {
		panic(err)
	}
	// 多家族场景：关系唯一约束需按家族隔离。旧库 person_relations 的
	// UNIQUE(from,to,type,custom) 是全局的，会导致同一对关系无法在不同家族各自维护。
	var tableSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='person_relations'`).Scan(&tableSQL); err == nil && strings.Contains(tableSQL, "UNIQUE(") && !strings.Contains(tableSQL, "UNIQUE(family_id") && !strings.Contains(tableSQL, "UNIQUE (family_id") {
		fmt.Println("检测到旧版 person_relations 唯一约束（未按家族隔离），正在重建表……")
		if _, err := db.Exec(`ALTER TABLE person_relations RENAME TO person_relations_old`); err != nil {
			panic(err)
		}
		schema := `CREATE TABLE person_relations (
  id TEXT PRIMARY KEY,
  family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
  from_person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  to_person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL,
  custom_name TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  inverse_id TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL,
  UNIQUE(family_id, from_person_id, to_person_id, relation_type, custom_name)
)`
		if _, err := db.Exec(schema); err != nil {
			panic(err)
		}
		if _, err := db.Exec(`INSERT INTO person_relations SELECT id,family_id,from_person_id,to_person_id,relation_type,COALESCE(custom_name,''),COALESCE(note,''),COALESCE(inverse_id,''),created_by,COALESCE(created_at,'') FROM person_relations_old`); err != nil {
			panic(err)
		}
		if _, err := db.Exec(`DROP TABLE person_relations_old`); err != nil {
			panic(err)
		}
		fmt.Println("person_relations 已按家族隔离重建")
	}
	fmt.Printf("database migrated: %s\n", dbPath)
}
