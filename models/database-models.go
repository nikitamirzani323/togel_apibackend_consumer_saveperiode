package models

import (
	"context"
	"database/sql"
	"log"
	"strings"
	s "strings"

	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/db"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/helpers"
)

func Get_listitemsearch(data, pemisah, search string) bool {
	flag := false
	temp := s.Split(data, pemisah)
	for i := 0; i < len(temp); i++ {
		if temp[i] == search {
			flag = true
			break
		}
	}
	return flag
}
func CheckDB(table, field, value string) bool {
	con := db.CreateCon()
	ctx := context.Background()
	flag := false
	sql_db := `SELECT 
					` + field + ` 
					FROM ` + table + ` 
					WHERE ` + field + ` = ? 
				`
	row := con.QueryRowContext(ctx, sql_db, value)
	switch e := row.Scan(&field); e {
	case sql.ErrNoRows:
		log.Println("No rows were returned!")
		flag = false
	case nil:
		flag = true
	default:
		panic(e)
	}
	return flag
}
func CheckDBTwoField(table, field_1, value_1, field_2, value_2 string) bool {
	con := db.CreateCon()
	ctx := context.Background()
	flag := false
	sql_db := `SELECT 
					` + field_1 + ` 
					FROM ` + table + ` 
					WHERE ` + field_1 + ` = ? 
					AND ` + field_2 + ` = ? 
				`
	log.Println(sql_db)
	row := con.QueryRowContext(ctx, sql_db, value_1, value_2)
	switch e := row.Scan(&field_1); e {
	case sql.ErrNoRows:
		log.Println("No rows were returned!")
		flag = false
	case nil:
		flag = true
	default:
		flag = false
	}
	return flag
}
func Get_mappingdatabase(company string) (string, string, string) {
	tbl_trx_keluarantogel := "db_tot_" + strings.ToLower(company) + ".tbl_trx_keluarantogel"
	tbl_trx_keluarantogel_detail := "db_tot_" + strings.ToLower(company) + ".tbl_trx_keluarantogel_detail"
	tbl_trx_keluarantogel_member := "db_tot_" + strings.ToLower(company) + ".tbl_trx_keluarantogel_member"

	return tbl_trx_keluarantogel, tbl_trx_keluarantogel_detail, tbl_trx_keluarantogel_member
}

func Exec_SQL(sql, table, action string, args ...interface{}) (bool, string) {
	con := db.CreateCon()
	ctx := context.Background()
	flag := false
	msg := ""
	stmt_exec, e_exec := con.PrepareContext(ctx, sql)
	helpers.ErrorCheck(e_exec)
	defer stmt_exec.Close()
	rec_exec, e_exec := stmt_exec.ExecContext(ctx, args...)

	helpers.ErrorCheck(e_exec)
	exec, e := rec_exec.RowsAffected()
	helpers.ErrorCheck(e)
	if exec > 0 {
		flag = true
		msg = "Data " + table + " Berhasil di " + action
	} else {
		msg = "Data " + table + " Failed di " + action
	}
	return flag, msg
}
