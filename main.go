package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/config"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/db"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/helpers"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/models"
	"github.com/nleeper/goment"
	amqp "github.com/rabbitmq/amqp091-go"
)

type datajobs struct {
	Idtrxkeluaran            string
	Idtrxkeluarandetail      string
	Statuskeluarandetail     string
	Updatekeluarandetail     string
	Updatedatekeluarandetail string
}
type dataresult struct {
	Idtrxkeluarandetail string
	Message             string
	Status              string
}

var mutex sync.RWMutex

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
func main() {
	db.Init()
	con := db.CreateCon()
	ctx := context.Background()
	AMPQ := os.Getenv("AMQP_SERVER_URL")
	conn, err := amqp.Dial(AMPQ)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"agensaveperiode", // name
		false,             // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{}
	// totalWorker := 100
	go func() {
		for d := range msgs {
			tglnow, _ := goment.New()
			json := []byte(d.Body)
			jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				Idtrxkeluaran, _, _, _ := jsonparser.Get(value, "Idtrxkeluaran")
				Idcomppasaran, _, _, _ := jsonparser.Get(value, "Idcomppasaran")
				Company, _, _, _ := jsonparser.Get(value, "Company")
				Keluarantogel, _, _, _ := jsonparser.Get(value, "Keluarantogel")
				Agent, _, _, _ := jsonparser.Get(value, "Agent")

				// total_record, _ := strconv.Atoi(string(Totaldata))
				// log.Printf("%s", total_record)
				// log.Println(string(Idtrxkeluaran))
				// log.Println(string(Company))
				flag := false
				tbl_trx_keluarantogel, tbl_trx_keluarantogel_detail, tbl_trx_keluarantogel_member := models.Get_mappingdatabase(string(Company))
				// log.Println(string(Idtrxkeluaran))
				// log.Println(string(Company))
				// log.Println(tbl_trx_keluarantogel)
				flag = models.CheckDBTwoField(tbl_trx_keluarantogel, "idtrxkeluaran", string(Idtrxkeluaran), "idcompany", string(Company))
				flag_next := true
				idinvoice, _ := strconv.Atoi(string(Idtrxkeluaran))
				idcomppasaran, _ := strconv.Atoi(string(Idcomppasaran))
				if flag {
					render_page := time.Now()
					runtime.GOMAXPROCS(8)
					totalWorker := 100
					totals_bet := _togel_bet_SUM_RUNNING(idinvoice, string(Company))
					buffer_bet := totals_bet + 1
					jobs_bet := make(chan datajobs, buffer_bet)
					results_bet := make(chan dataresult, buffer_bet)

					wg := &sync.WaitGroup{}
					for w := 0; w < totalWorker; w++ {
						wg.Add(1)
						mutex.Lock()
						go _doJobUpdateTransaksi(tbl_trx_keluarantogel_detail, jobs_bet, results_bet, wg)
						mutex.Unlock()
					}
					sql_detailbet := `SELECT 
						idtrxkeluarandetail, typegame, bet, diskon, kei, nomortogel, posisitogel    
						FROM ` + tbl_trx_keluarantogel_detail + `  
						WHERE idtrxkeluaran = ? 
						AND idcompany = ? 
						AND statuskeluarandetail = "RUNNING" 
					`

					row_detailbet, err_detailbet := con.QueryContext(ctx, sql_detailbet, string(Idtrxkeluaran), string(Company))
					for row_detailbet.Next() {
						mutex.Lock()
						var (
							idtrxkeluarandetail_db                     int
							typegame_db, nomortogel_db, posisitogel_db string
							diskon_db, kei_db, bet_db                  float32
						)

						err_detailbet = row_detailbet.Scan(
							&idtrxkeluarandetail_db,
							&typegame_db,
							&bet_db,
							&diskon_db,
							&kei_db,
							&nomortogel_db, &posisitogel_db)

						helpers.ErrorCheck(err_detailbet)
						statuskeluarandetail, _ := _rumusTogel(string(Keluarantogel), typegame_db, nomortogel_db, posisitogel_db, string(Company), "Y", idcomppasaran, idtrxkeluarandetail_db)
						jobs_bet <- datajobs{
							Idtrxkeluarandetail:      strconv.Itoa(idtrxkeluarandetail_db),
							Idtrxkeluaran:            strconv.Itoa(idinvoice),
							Statuskeluarandetail:     statuskeluarandetail,
							Updatekeluarandetail:     string(Agent),
							Updatedatekeluarandetail: tglnow.Format("YYYY-MM-DD HH:mm:ss"),
						}
						mutex.Unlock()
					}
					defer row_detailbet.Close()
					close(jobs_bet)
					for a := 1; a <= totals_bet; a++ { //RESULT
						flag_result := <-results_bet
						if flag_result.Status == "Failed" {
							flag_next = false
							log.Printf("ID : %s, Message: %s", flag_result.Idtrxkeluarandetail, flag_result.Message)
						}

					}
					close(results_bet)
					wg.Wait()
					log.Println("TIME JOBS: ", time.Since(render_page).String())
					log.Println("FLAGS: ", flag)
				}
				if flag_next {
					sql_detailbetwinner := `SELECT
						idtrxkeluarandetail, username, typegame, bet, diskon, kei, win 
						FROM ` + tbl_trx_keluarantogel_detail + `
						WHERE idtrxkeluaran = ?
						AND idcompany = ?
						AND statuskeluarandetail = "WINNER"
					`
					row_detailbetwinner, err_detailbetwinner := con.QueryContext(ctx, sql_detailbetwinner, string(Idtrxkeluaran), string(Company))

					helpers.ErrorCheck(err_detailbetwinner)
					totalmembertogel := _togel_member_COUNT(idinvoice, string(Company))
					totalbet := _togel_bet_SUM(idinvoice, string(Company))
					totalbayar := _togel_bayar_SUM(idinvoice, string(Company))
					totalwin := 0
					for row_detailbetwinner.Next() {
						var (
							idtrxkeluarandetail_db2           int
							username_db, typegame_db          string
							diskon_db, kei_db, win_db, bet_db float32
						)

						err_detailbetwinner = row_detailbetwinner.Scan(
							&idtrxkeluarandetail_db2,
							&username_db,
							&typegame_db,
							&bet_db,
							&diskon_db,
							&kei_db,
							&win_db)
						bayar := int(bet_db) - int(float32(bet_db)*diskon_db) - int(float32(bet_db)*kei_db)
						winhasil := _rumuswinhasil(typegame_db, bayar, int(bet_db), win_db)
						totalwin = totalwin + winhasil

						//UPDATE DETAIL KELUARAN MEMBER WINHASIL
						sql_update := `
							UPDATE
							` + tbl_trx_keluarantogel_detail + `
							SET winhasil=? ,
							updatekeluarandetail=?, updatedatekeluarandetail=?
							WHERE idtrxkeluarandetail=?  AND idtrxkeluaran=? AND username=?
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							winhasil,
							string(Agent),
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail_db2, string(Idtrxkeluaran), username_db)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
					defer row_detailbetwinner.Close()

					//UPDATE CANCELBET DI tbl_trx_keluarantogel_detail
					sql_detailbetcancel := `SELECT
						idtrxkeluarandetail, username, typegame, bet, diskon, kei, win
						FROM ` + tbl_trx_keluarantogel_detail + `
						WHERE idtrxkeluaran = ?
						AND idcompany = ?
						AND statuskeluarandetail = "CANCEL"
					`
					row_detailbetcancel, err_detailbetcancel := con.QueryContext(ctx, sql_detailbetcancel, string(Idtrxkeluaran), string(Company))

					helpers.ErrorCheck(err_detailbetcancel)
					totalcancel := 0
					for row_detailbetcancel.Next() {
						var (
							idtrxkeluarandetail_db2           int
							username_db, typegame_db          string
							diskon_db, kei_db, win_db, bet_db float32
						)

						err_detailbetcancel = row_detailbetcancel.Scan(
							&idtrxkeluarandetail_db2,
							&username_db,
							&typegame_db,
							&bet_db,
							&diskon_db,
							&kei_db,
							&win_db)
						bayar := int(bet_db) - int(float32(bet_db)*diskon_db) - int(float32(bet_db)*kei_db)
						totalcancel = totalcancel + bayar

						//UPDATE DETAIL KELUARAN MEMBER CANCELBET
						sql_update := `
							UPDATE
							` + tbl_trx_keluarantogel_detail + `
							SET cancelbet=? ,
							updatekeluarandetail=?, updatedatekeluarandetail=?
							WHERE idtrxkeluarandetail=?  AND idtrxkeluaran=? AND username=?
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							bayar,
							string(Agent),
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail_db2, string(Idtrxkeluaran), username_db)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}

					}
					defer row_detailbetwinner.Close()

					log.Printf("TOTAL BET: %d - TOTAL BAYAR: %d - TOTAL WIN: %d - TOTAL MEMBER:%d - TOTAL CANCEL:%d", totalbet, totalbayar, totalwin, totalmembertogel, totalcancel)
					if totalbet > 0 {
						//UPDATE DETAIL KELUARAN
						sql_update := `
							UPDATE
							` + tbl_trx_keluarantogel + `
							SET total_bet=? , total_outstanding=?, winlose=?, total_member=?, total_cancel=?, 
							updatekeluaran=?, updatedatekeluaran=?
							WHERE idtrxkeluaran=?
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							totalbet,
							totalbayar,
							totalwin,
							totalmembertogel,
							totalcancel,
							string(Agent),
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							string(Idtrxkeluaran))
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}

					totalmember_tblmember := _togel_totalmember_COUNT(idinvoice, string(Company))

					if totalmember_tblmember < 1 {
						year := tglnow.Format("YYYY")
						month := tglnow.Format("MM")
						// INSERT TABLE TOGEL MEMBER
						sql_detailgroupmember := `SELECT
							username,
							count(username) as totalbet,
							sum(bet-(bet*diskon)-(bet*kei)) as totalbayar,
							sum(winhasil) as totalwin, 
							sum(cancelbet) as totalcancel 
							FROM ` + tbl_trx_keluarantogel_detail + `
							WHERE idtrxkeluaran = ?
							AND idcompany = ?
							GROUP BY username
						`
						row_detailgroupmember, err_detailgroupmember := con.QueryContext(ctx, sql_detailgroupmember, string(Idtrxkeluaran), string(Company))
						helpers.ErrorCheck(err_detailgroupmember)
						for row_detailgroupmember.Next() {
							var (
								totalbet_db, totalbayar_db, totalwin_db, totalcancel_db int
								username_db                                             string
							)

							err_detailgroupmember = row_detailgroupmember.Scan(
								&username_db,
								&totalbet_db,
								&totalbayar_db,
								&totalwin_db, &totalcancel_db)

							field_col2 := tbl_trx_keluarantogel_member + year + month
							idkeluaranmember_counter := models.Get_counter(field_col2)
							idkeluaranmember := year + month + strconv.Itoa(idkeluaranmember_counter)
							//INSERT TABLE KELUARAN TOGEL MEMBER
							sql_insert := `
								insert into
								` + tbl_trx_keluarantogel_member + ` (
									idkeluaranmember, idtrxkeluaran, idcompany,
									username, totalbet, totalbayar, totalwin, totalcancel, 
									createkeluaranmember, createdatekeluaranmember
								) values (
									?, ?, ?,
									?, ?, ?, ?, ?, 
									?, ?
								)
							`
							flag_insert, msg_insert := models.Exec_SQL(sql_insert, tbl_trx_keluarantogel_member, "INSERT",
								idkeluaranmember,
								string(Idtrxkeluaran),
								string(Company),
								username_db,
								totalbet_db,
								totalbayar_db,
								totalwin_db,
								totalcancel_db,
								string(Agent),
								tglnow.Format("YYYY-MM-DD HH:mm:ss"))
							if flag_insert {
								log.Println(msg_insert)
							} else {
								log.Println(msg_insert)
							}
						}
						defer row_detailgroupmember.Close()
					} else {
						log.Println("Data Member tbl_trx_keluarantogel_member Failed ")
					}
				}
			})
		}
	}()

	log.Printf(" [*] Waiting for messages agen saveperiode. To exit press CTRL+C")
	<-forever
}

func _doJobUpdateTransaksi(fieldtable string, jobs <-chan datajobs, results chan<- dataresult, wg *sync.WaitGroup) {
	for capture := range jobs {
		for {
			var outerError error
			func(outerError *error) {
				defer func() {
					if err := recover(); err != nil {
						*outerError = fmt.Errorf("%v", err)
					}
				}()

				sql_update := `
				UPDATE 
				` + fieldtable + `      
				SET statuskeluarandetail=? , 
				updatekeluarandetail=?, updatedatekeluarandetail=? 
				WHERE idtrxkeluarandetail=?  AND idtrxkeluaran=? 
				`
				flag_update, msg_update := models.Exec_SQL(sql_update, fieldtable, "UPDATE",
					capture.Statuskeluarandetail,
					capture.Updatekeluarandetail,
					capture.Updatedatekeluarandetail,
					capture.Idtrxkeluarandetail, capture.Idtrxkeluaran)

				if flag_update {
					results <- dataresult{Idtrxkeluarandetail: capture.Idtrxkeluarandetail, Message: "Tidak ada masalah", Status: "Success"}
				} else {
					results <- dataresult{Idtrxkeluarandetail: capture.Idtrxkeluarandetail, Message: msg_update, Status: "Failed"}
				}

			}(&outerError)
			if outerError == nil {
				break
			}
		}
	}
	wg.Done()
}
func _deleteredis_generator(company string, idtrxkeluaran int) {
	const Fieldperiode_home_redis = "LISTPERIODE_AGENT_"
	log.Println("REDIS DELETE")
	log.Println("COMPANY :", company)
	log.Println("INVOICE :", idtrxkeluaran)

	//AGEN
	field_home_redis := Fieldperiode_home_redis + strings.ToLower(company)
	val_homeredis := helpers.DeleteRedis(field_home_redis)
	log.Printf("Redis Delete AGEN - PERIODE HOME : %d", val_homeredis)

	field_homedetail_redis := Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran)
	val_homedetailredis := helpers.DeleteRedis(field_homedetail_redis)
	log.Printf("%s\n", field_homedetail_redis)
	log.Printf("Redis Delete AGEN - PERIODE DETAIL : %d", val_homedetailredis)

	field_homedetail_listmember_redis := Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTMEMBER"
	val_homedetaillistmember_redis := helpers.DeleteRedis(field_homedetail_listmember_redis)
	log.Printf("%s\n", field_homedetail_listmember_redis)
	log.Printf("Redis Delete AGEN - PERIODE DETAIL LISTMEMBER : %d", val_homedetaillistmember_redis)

	field_homedetail_listbettable_redis := Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTBETTABLE"
	val_homedetaillistbettable_redis := helpers.DeleteRedis(field_homedetail_listbettable_redis)
	log.Printf("Redis Delete AGEN - PERIODE DETAIL LISTBETTABLE : %d", val_homedetaillistbettable_redis)

	log_redis := "LISTLOG_AGENT_" + strings.ToLower(company)
	val_agent_redis := helpers.DeleteRedis(log_redis)
	log.Printf("Redis Delete AGEN - LOG status: %d", val_agent_redis)

	val_agent_dashboard := helpers.DeleteRedis("DASHBOARD_CHART_AGENT_" + strings.ToLower(company))
	log.Printf("Redis Delete AGENT DASHBOARD status: %d", val_agent_dashboard)

	val_agent_dashboard_pasaranhome := helpers.DeleteRedis("LISTDASHBOARDPASARAN_AGENT_" + strings.ToLower(company))
	log.Printf("Redis Delete AGENT DASHBOARD PASARAN status: %d", val_agent_dashboard_pasaranhome)

	val_agent4d := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_4D")
	val_agent3d := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_3D")
	val_agent2d := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_2D")
	val_agent2dd := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_2DD")
	val_agent2dt := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_2DT")
	val_agentcolokbebas := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_COLOK_BEBAS")
	val_agentcolokmacau := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_COLOK_MACAU")
	val_agentcoloknaga := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_COLOK_NAGA")
	val_agentcolokjitu := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_COLOK_JITU")
	val_agent5050umum := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_50_50_UMUM")
	val_agent5050special := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_50_50_SPECIAL")
	val_agent5050kombinasi := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_50_50_KOMBINASI")
	val_agentmacaukombinasi := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_MACAU_KOMBINASI")
	val_agentdasar := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_DASAR")
	val_agentshio := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTPERMAINAN_SHIO")
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 4D: %d", val_agent4d)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 3D: %d", val_agent3d)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 2D: %d", val_agent2d)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 2DD: %d", val_agent2dd)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 2DT: %d", val_agent2dt)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET COLOK BEBAS: %d", val_agentcolokbebas)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET COLOK MACAU: %d", val_agentcolokmacau)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET COLOK NAGA: %d", val_agentcoloknaga)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET COLOK JITU: %d", val_agentcolokjitu)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 5050UMUM: %d", val_agent5050umum)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 5050SPECIAL: %d", val_agent5050special)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET 5050KOMBINASI: %d", val_agent5050kombinasi)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET MACAU KOMBINASI: %d", val_agentmacaukombinasi)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET DASAR: %d", val_agentdasar)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET SHIO: %d", val_agentshio)
	val_agentall := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTBET_all")
	val_agentwinner := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTBET_winner")
	val_agentcancel := helpers.DeleteRedis(Fieldperiode_home_redis + strings.ToLower(company) + "_INVOICE_" + strconv.Itoa(idtrxkeluaran) + "_LISTBET_cancel")
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET STATUS ALL: %d", val_agentall)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET STATUS WINNER: %d", val_agentwinner)
	log.Printf("Redis Delete AGENT PERIODE DETAIL LIST BET STATUS CANCEL: %d", val_agentcancel)

}
func _rumuswinhasil(permainan string, bayar int, bet int, win float32) int {
	winhasil := 0
	if permainan == "50_50_UMUM" || permainan == "50_50_SPECIAL" ||
		permainan == "50_50_KOMBINASI" || permainan == "DASAR" || permainan == "COLOK_BEBAS" ||
		permainan == "COLOK_MACAU" || permainan == "COLOK_NAGA" || permainan == "COLOK_JITU" {

		winhasil = bayar + int(float32(bet)*win)
	} else {
		winhasil = int(float32(bet) * win)
	}
	return winhasil
}
func _togel_bet_SUM_RUNNING(idtrxkeluaran int, company string) int {
	con := db.CreateCon()
	ctx := context.Background()

	_, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(company)

	sql_bet := `SELECT 
		count(idtrxkeluarandetail) as total  
		FROM ` + tbl_trx_keluarantogel_detail + `   
		WHERE idcompany = ? 
		AND idtrxkeluaran = ? 
		AND statuskeluarandetail = "RUNNING" 
	`
	var total_db sql.NullInt32
	total := 0

	row := con.QueryRowContext(ctx, sql_bet, company, idtrxkeluaran)
	switch e := row.Scan(&total_db); e {
	case sql.ErrNoRows:
		log.Println("No rows were returned!")
	case nil:
		log.Println(total_db)
	default:
		log.Panic(e)
	}
	if total_db.Valid {
		total = int(total_db.Int32)
	}
	return total
}
func _togel_bayar_SUM(idtrxkeluaran int, company string) int {
	con := db.CreateCon()
	ctx := context.Background()

	_, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(company)

	sql_bayar := `SELECT 
		sum(bet-(bet*diskon)-(bet*kei)) as total  
		FROM ` + tbl_trx_keluarantogel_detail + `   
		WHERE idcompany = ? 
		AND idtrxkeluaran = ? 
	`
	var total_db sql.NullFloat64
	total := 0
	row := con.QueryRowContext(ctx, sql_bayar, company, idtrxkeluaran)
	switch e := row.Scan(&total_db); e {
	case sql.ErrNoRows:
		log.Println("No rows were returned!")
	case nil:
		log.Println(total_db)
	default:
		log.Panic(e)
	}
	if total_db.Valid {
		total = int(total_db.Float64)
	}
	return total
}
func _togel_bet_SUM(idtrxkeluaran int, company string) int {
	con := db.CreateCon()
	ctx := context.Background()

	_, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(company)

	sql_bet := `SELECT 
		count(idtrxkeluarandetail) as total  
		FROM ` + tbl_trx_keluarantogel_detail + `   
		WHERE idcompany = ? 
		AND idtrxkeluaran = ? 
	`
	var total_db sql.NullInt32
	total := 0

	row := con.QueryRowContext(ctx, sql_bet, company, idtrxkeluaran)
	switch e := row.Scan(&total_db); e {
	case sql.ErrNoRows:
		log.Println("No rows were returned!")
	case nil:
		log.Println(total_db)
	default:
		log.Panic(e)
	}
	if total_db.Valid {
		total = int(total_db.Int32)
	}
	return total
}
func _togel_member_COUNT(idtrxkeluaran int, company string) int {
	con := db.CreateCon()
	ctx := context.Background()
	total := 0

	_, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(company)

	sql := `SELECT 
		username
		FROM ` + tbl_trx_keluarantogel_detail + `  
		WHERE idcompany = ? 
		AND idtrxkeluaran = ? 
		GROUP BY username 
	`

	row, err := con.QueryContext(ctx, sql, company, idtrxkeluaran)

	helpers.ErrorCheck(err)
	for row.Next() {
		total = total + 1
		var username_db string

		err = row.Scan(
			&username_db)

		helpers.ErrorCheck(err)

	}
	defer row.Close()

	return total
}
func _togel_totalmember_COUNT(idtrxkeluaran int, company string) int {
	con := db.CreateCon()
	ctx := context.Background()
	total := 0
	_, _, tbl_trx_keluarantogel_member := models.Get_mappingdatabase(company)

	sql := `SELECT 
		idkeluaranmember
		FROM ` + tbl_trx_keluarantogel_member + `   
		WHERE idcompany = ? 
		AND idtrxkeluaran = ? 
	`

	row, err := con.QueryContext(ctx, sql, company, idtrxkeluaran)

	helpers.ErrorCheck(err)
	for row.Next() {
		total = total + 1
		var idkeluaranmember_db int

		err = row.Scan(
			&idkeluaranmember_db)

		helpers.ErrorCheck(err)

	}
	defer row.Close()

	return total
}
func _rumusTogel(angka, tipe, nomorkeluaran, posisitogel, company, simpandb string, idcomppasaran, idtrxkeluarandetail int) (string, float32) {
	tglnow, _ := goment.New()
	var result string = "LOSE"
	var win float32 = 0

	_, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(company)

	temp := angka
	temp4d := string([]byte(temp)[0]) + string([]byte(temp)[1]) + string([]byte(temp)[2]) + string([]byte(temp)[3])
	temp3d := string([]byte(temp)[1]) + string([]byte(temp)[2]) + string([]byte(temp)[3])
	temp3dd := string([]byte(temp)[0]) + string([]byte(temp)[1]) + string([]byte(temp)[2])
	temp2d := string([]byte(temp)[2]) + string([]byte(temp)[3])
	temp2dd := string([]byte(temp)[0]) + string([]byte(temp)[1])
	temp2dt := string([]byte(temp)[1]) + string([]byte(temp)[2])

	var temp4d_arr []string
	var temp3d_arr []string
	var temp3dd_arr []string
	var temp2d_arr []string
	var temp2dd_arr []string
	var temp2dt_arr []string

	switch tipe {
	case "4D":
		if temp4d == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_4D := false
				for a, _ := range temp4d {
					for b, _ := range temp4d {
						for c, _ := range temp4d {
							for d, _ := range temp4d {
								if string([]byte(temp4d)[a]) != string([]byte(temp4d)[b]) && string([]byte(temp4d)[a]) != string([]byte(temp4d)[c]) && string([]byte(temp4d)[a]) != string([]byte(temp4d)[d]) {
									if string([]byte(temp4d)[b]) != string([]byte(temp4d)[c]) && string([]byte(temp4d)[b]) != string([]byte(temp4d)[d]) {
										if string([]byte(temp4d)[c]) != string([]byte(temp4d)[d]) {
											temp_loop := string([]byte(temp4d)[a]) + string([]byte(temp4d)[b]) + string([]byte(temp4d)[c]) + string([]byte(temp4d)[d])
											if temp4d != temp_loop {
												temp4d_arr = append(temp4d_arr, temp_loop)
											}
										}
									}
								}
							}
						}
					}
				}
				for a, _ := range temp4d_arr {
					if temp4d_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_4D = true
					}

				}
				if flag_bb_4D {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win4dbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "3D":
		if temp3d == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_3D := false
				for a, _ := range temp3d {
					for b, _ := range temp3d {
						for c, _ := range temp3d {
							if string([]byte(temp3d)[a]) != string([]byte(temp3d)[b]) && string([]byte(temp3d)[a]) != string([]byte(temp3d)[c]) {
								if string([]byte(temp3d)[b]) != string([]byte(temp3d)[c]) {
									temp_loop := string([]byte(temp3d)[a]) + string([]byte(temp3d)[b]) + string([]byte(temp3d)[c])
									if temp3d != temp_loop {
										temp3d_arr = append(temp3d_arr, temp_loop)
									}
								}
							}

						}
					}
				}
				for a, _ := range temp3d_arr {
					if temp3d_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_3D = true
					}

				}
				if flag_bb_3D {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win3dbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "3DD":
		if temp3dd == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_3DD := false
				for a, _ := range temp3dd {
					for b, _ := range temp3dd {
						for c, _ := range temp3dd {
							if string([]byte(temp3dd)[a]) != string([]byte(temp3dd)[b]) && string([]byte(temp3dd)[a]) != string([]byte(temp3dd)[c]) {
								if string([]byte(temp3dd)[b]) != string([]byte(temp3dd)[c]) {
									temp_loop := string([]byte(temp3dd)[a]) + string([]byte(temp3dd)[b]) + string([]byte(temp3dd)[c])
									if temp3dd != temp_loop {
										temp3dd_arr = append(temp3dd_arr, temp_loop)
									}
								}
							}

						}
					}
				}
				for a, _ := range temp3dd_arr {
					if temp3dd_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_3DD = true
					}

				}
				if flag_bb_3DD {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win3ddbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "2D":
		if temp2d == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_2D := false
				for a, _ := range temp2d {
					for b, _ := range temp2d {
						if string([]byte(temp2d)[a]) != string([]byte(temp2d)[b]) {
							temp_loop := string([]byte(temp2d)[a]) + string([]byte(temp2d)[b])
							if temp2d != temp_loop {
								temp2d_arr = append(temp2d_arr, temp_loop)
							}
						}
					}
				}
				for a, _ := range temp2d_arr {
					if temp2d_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_2D = true
					}

				}
				if flag_bb_2D {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win2dbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "2DD":
		if temp2dd == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_2DD := false
				for a, _ := range temp2dd {
					for b, _ := range temp2dd {
						if string([]byte(temp2dd)[a]) != string([]byte(temp2dd)[b]) {
							temp_loop := string([]byte(temp2dd)[a]) + string([]byte(temp2dd)[b])
							if temp2dd != temp_loop {
								temp2dd_arr = append(temp2dd_arr, temp_loop)
							}
						}
					}
				}
				for a, _ := range temp2dd_arr {
					if temp2dd_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_2DD = true
					}

				}
				if flag_bb_2DD {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win2ddbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "2DT":
		if temp2dt == nomorkeluaran {
			result = "WINNER"
		} else {
			if posisitogel == "BB" {
				flag_bb_2DT := false
				for a, _ := range temp2dt {
					for b, _ := range temp2dt {
						if string([]byte(temp2dt)[a]) != string([]byte(temp2dt)[b]) {
							temp_loop := string([]byte(temp2dt)[a]) + string([]byte(temp2dt)[b])
							if temp2dt != temp_loop {
								temp2dt_arr = append(temp2dt_arr, temp_loop)
							}
						}
					}
				}
				for a, _ := range temp2dt_arr {
					if temp2dt_arr[a] == nomorkeluaran {
						result = "WINNER"
						flag_bb_2DT = true
					}

				}
				if flag_bb_2DT {
					_, win_db := Pasaran_id(idcomppasaran, company, "1_win2dtbb")
					win = win_db
					if simpandb == "Y" {
						sql_update := `
							UPDATE 
							` + tbl_trx_keluarantogel_detail + `     
							SET win=?, 
							updatekeluarandetail=?, updatedatekeluarandetail=? 
							WHERE idtrxkeluarandetail=? 
						`
						flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
							win_db,
							"SYSTEM",
							tglnow.Format("YYYY-MM-DD HH:mm:ss"),
							idtrxkeluarandetail)
						if flag_update {
							log.Println(msg_update)
						} else {
							log.Println(msg_update)
						}
					}
				}
			}
		}
	case "COLOK_BEBAS":
		flag := false
		count := 0
		for i := 0; i < len(temp); i++ {
			if string([]byte(temp)[i]) == nomorkeluaran {
				flag = true
				count = count + 1
			}
		}
		if flag {
			_, win_db := Pasaran_id(idcomppasaran, company, "2_win")
			if count == 1 {
				win = win_db
			}
			if count == 2 {
				win = win_db * 2
			}
			if count == 3 {
				win = win_db * 3
			}
			if count == 4 {
				win = win_db * 3
			}
			fmt.Println(win)

			if simpandb == "Y" {
				//UPDATE WIN DETAIL BET
				sql_update := `
					UPDATE 
					` + tbl_trx_keluarantogel_detail + `     
					SET win=?, 
					updatekeluarandetail=?, updatedatekeluarandetail=? 
					WHERE idtrxkeluarandetail=? 
				`
				flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
					win,
					"SYSTEM",
					tglnow.Format("YYYY-MM-DD HH:mm:ss"),
					idtrxkeluarandetail)
				if flag_update {
					log.Println(msg_update)
				} else {
					log.Println(msg_update)
				}
			}
			result = "WINNER"
		}
	case "COLOK_MACAU":
		flag_1 := false
		flag_2 := false
		count_1 := 0
		count_2 := 0
		totalcount := 0
		var win float32 = 0
		for i := 0; i < len(temp); i++ {
			if string([]byte(temp)[i]) == string([]byte(nomorkeluaran)[0]) {
				flag_1 = true
				count_1 = count_1 + 1
			}
			if string([]byte(temp)[i]) == string([]byte(nomorkeluaran)[1]) {
				flag_2 = true
				count_2 = count_2 + 1
			}
		}
		if flag_1 && flag_2 {
			totalcount = count_1 + count_2
			if totalcount == 2 {
				_, win = Pasaran_id(idcomppasaran, company, "3_win2digit")
			}
			if totalcount == 3 {
				_, win = Pasaran_id(idcomppasaran, company, "3_win3digit")
			}
			if totalcount == 4 {
				_, win = Pasaran_id(idcomppasaran, company, "3_win4digit")
			}
			if simpandb == "Y" {
				//UPDATE WIN DETAIL BET
				sql_update := `
					UPDATE 
					` + tbl_trx_keluarantogel_detail + `     
					SET win=?, 
					updatekeluarandetail=?, updatedatekeluarandetail=? 
					WHERE idtrxkeluarandetail=? 
				`
				flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
					win,
					"SYSTEM",
					tglnow.Format("YYYY-MM-DD HH:mm:ss"),
					idtrxkeluarandetail)
				if flag_update {
					log.Println(msg_update)
				} else {
					log.Println(msg_update)
				}
			}
			result = "WINNER"
		}
	case "COLOK_NAGA":
		flag_1 := false
		flag_2 := false
		flag_3 := false
		count_1 := 0
		count_2 := 0
		count_3 := 0
		totalcount := 0
		var win float32 = 0
		for i := 0; i < len(temp); i++ {
			if string([]byte(temp)[i]) == string([]byte(nomorkeluaran)[0]) {
				flag_1 = true
				count_1 = count_1 + 1
			}
			if string([]byte(temp)[i]) == string([]byte(nomorkeluaran)[1]) {
				flag_2 = true
				count_2 = count_2 + 1
			}
			if string([]byte(temp)[i]) == string([]byte(nomorkeluaran)[2]) {
				flag_3 = true
				count_3 = count_3 + 1
			}
		}
		if flag_1 && flag_2 {
			if flag_3 {
				totalcount = count_1 + count_2 + count_3
				log.Println("Total Count Colok Naga :", totalcount)

				if totalcount == 3 {
					_, win = Pasaran_id(idcomppasaran, company, "4_win3digit")
				}
				if totalcount == 4 {
					_, win = Pasaran_id(idcomppasaran, company, "4_win4digit")
				}
				log.Println("WIN COLOK NAGA :", win)
				if simpandb == "Y" {
					//UPDATE WIN DETAIL BET
					sql_update := `
						UPDATE 
						` + tbl_trx_keluarantogel_detail + `     
						SET win=?,  
						updatekeluarandetail=?, updatedatekeluarandetail=? 
						WHERE idtrxkeluarandetail=? 
					`
					flag_update, msg_update := models.Exec_SQL(sql_update, tbl_trx_keluarantogel_detail, "UPDATE",
						win,
						"SYSTEM",
						tglnow.Format("YYYY-MM-DD HH:mm:ss"),
						idtrxkeluarandetail)
					if flag_update {
						log.Println(msg_update)
					} else {
						log.Println(msg_update)
					}
				}
				result = "WINNER"
			}
		}
	case "COLOK_JITU":
		flag := false
		as := string([]byte(temp)[0]) + "_AS"
		kop := string([]byte(temp)[1]) + "_KOP"
		kepala := string([]byte(temp)[2]) + "_KEPALA"
		ekor := string([]byte(temp)[3]) + "_EKOR"

		if as == nomorkeluaran {
			flag = true
		}
		if kop == nomorkeluaran {
			flag = true
		}
		if kepala == nomorkeluaran {
			flag = true
		}
		if ekor == nomorkeluaran {
			flag = true
		}
		if flag {
			result = "WINNER"
		}
	case "50_50_UMUM":
		flag := false
		data := []string{}
		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])
		kepala_2, _ := strconv.Atoi(kepala)
		ekor_2, _ := strconv.Atoi(ekor)
		dasar, _ := strconv.Atoi(kepala + ekor)
		//BESARKECIL
		if kepala_2 <= 4 {
			data = append(data, "KECIL")
		} else {
			data = append(data, "BESAR")
		}
		//GENAPGANJIL
		if ekor_2%2 == 0 {
			data = append(data, "GENAP")
		} else {
			data = append(data, "GANJIL")
		}
		log.Printf("DASAR : %d", dasar)
		//TEPITENGAH
		if dasar >= 0 && dasar <= 24 {
			data = append(data, "TEPI")
		}
		if dasar >= 25 && dasar <= 74 {
			data = append(data, "TENGAH")
		}
		if dasar >= 75 && dasar <= 99 {
			data = append(data, "TEPI")
		}
		for i := 0; i < len(data); i++ {
			if data[i] == nomorkeluaran {
				flag = true
			}
		}
		if flag {
			result = "WINNER"
		}
		fmt.Println(data)
	case "50_50_SPECIAL":
		flag := false
		as := string([]byte(temp)[0])
		kop := string([]byte(temp)[1])
		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])

		as_2, _ := strconv.Atoi(as)
		kop_2, _ := strconv.Atoi(kop)
		kepala_2, _ := strconv.Atoi(kepala)
		ekor_2, _ := strconv.Atoi(ekor)
		//AS - BESARKECIL == GENAPGANJIL
		if as_2 <= 4 {
			if nomorkeluaran == "AS_KECIL" {
				flag = true
			}
		} else {
			if nomorkeluaran == "AS_BESAR" {
				flag = true
			}
		}
		if as_2%2 == 0 {
			if nomorkeluaran == "AS_GENAP" {
				flag = true
			}
		} else {
			if nomorkeluaran == "AS_GANJIL" {
				flag = true
			}
		}

		//KOP - BESARKECIL == GENAPGANJIL
		if kop_2 <= 4 {
			if nomorkeluaran == "KOP_KECIL" {
				flag = true
			}
		} else {
			if nomorkeluaran == "KOP_BESAR" {
				flag = true
			}
		}
		if kop_2%2 == 0 {
			if nomorkeluaran == "KOP_GENAP" {
				flag = true
			}
		} else {
			if nomorkeluaran == "KOP_GANJIL" {
				flag = true
			}
		}

		//KEPALA - BESARKECIL == GENAPGANJIL
		if kepala_2 <= 4 {
			if nomorkeluaran == "KEPALA_KECIL" {
				flag = true
			}
		} else {
			if nomorkeluaran == "KEPALA_BESAR" {
				flag = true
			}
		}
		if kepala_2%2 == 0 {
			if nomorkeluaran == "KEPALA_GENAP" {
				flag = true
			}
		} else {
			if nomorkeluaran == "KEPALA_GANJIL" {
				flag = true
			}
		}

		//EKOR - BESARKECIL == GENAPGANJIL
		if ekor_2 <= 4 {
			if nomorkeluaran == "EKOR_KECIL" {
				flag = true
			}
		} else {
			if nomorkeluaran == "EKOR_BESAR" {
				flag = true
			}
		}
		if ekor_2%2 == 0 {
			if nomorkeluaran == "EKOR_GENAP" {
				flag = true
			}
		} else {
			if nomorkeluaran == "EKOR_GANJIL" {
				flag = true
			}
		}

		if flag {
			result = "WINNER"
		}
	case "50_50_KOMBINASI":
		flag := false
		data_1 := ""
		data_2 := ""
		data_3 := ""
		data_4 := ""
		depan := ""
		tengah := ""
		belakang := ""
		depan_1 := ""
		tengah_1 := ""
		belakang_1 := ""
		as := string([]byte(temp)[0])
		kop := string([]byte(temp)[1])
		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])

		as_2, _ := strconv.Atoi(as)
		kop_2, _ := strconv.Atoi(kop)
		kepala_2, _ := strconv.Atoi(kepala)
		ekor_2, _ := strconv.Atoi(ekor)

		if as_2%2 == 0 {
			data_1 = "GENAP"
		} else {
			data_1 = "GANJIL"
		}
		if kop_2%2 == 0 {
			data_2 = "GENAP"
		} else {
			data_2 = "GANJIL"
		}
		if kepala_2%2 == 0 {
			data_3 = "GENAP"
		} else {
			data_3 = "GANJIL"
		}
		if ekor_2%2 == 0 {
			data_4 = "GENAP"
		} else {
			data_4 = "GANJIL"
		}
		depan = data_1 + "-" + data_2
		tengah = data_2 + "-" + data_3
		belakang = data_3 + "-" + data_4

		if depan == "GENAP-GANJIL" || depan == "GANJIL-GENAP" {
			depan = "DEPAN_STEREO"
		} else {
			depan = "DEPAN_MONO"
		}
		if tengah == "GENAP-GANJIL" || tengah == "GANJIL-GENAP" {
			tengah = "TENGAH_STEREO"
		} else {
			tengah = "TENGAH_MONO"
		}
		if belakang == "GENAP-GANJIL" || belakang == "GANJIL-GENAP" {
			belakang = "BELAKANG_STEREO"
		} else {
			belakang = "BELAKANG_MONO"
		}
		if as_2 < kop_2 {
			depan_1 = "DEPAN_KEMBANG"
		}
		if as_2 > kop_2 {
			depan_1 = "DEPAN_KEMPIS"
		}
		if as_2 == kop_2 {
			depan_1 = "DEPAN_KEMBAR"
		}
		if kop_2 < kepala_2 {
			tengah_1 = "TENGAH_KEMBANG"
		}
		if kop_2 > kepala_2 {
			tengah_1 = "TENGAH_KEMPIS"
		}
		if kop_2 == kepala_2 {
			tengah_1 = "TENGAH_KEMBAR"
		}
		if kepala_2 < ekor_2 {
			belakang_1 = "BELAKANG_KEMBANG"
		}
		if kepala_2 > ekor_2 {
			belakang_1 = "BELAKANG_KEMPIS"
		}
		if kepala_2 == ekor_2 {
			belakang_1 = "BELAKANG_KEMBAR"
		}

		if depan == nomorkeluaran {
			flag = true
		}
		if tengah == nomorkeluaran {
			flag = true
		}
		if belakang == nomorkeluaran {
			flag = true
		}
		if depan_1 == nomorkeluaran {
			flag = true
		}
		if tengah_1 == nomorkeluaran {
			flag = true
		}
		if belakang_1 == nomorkeluaran {
			flag = true
		}

		if flag {
			result = "WINNER"
		}
	case "MACAU_KOMBINASI":
		flag := false
		data_1 := ""
		data_2 := ""
		data_3 := ""
		data_4 := ""
		depan := ""
		tengah := ""
		tengah2 := ""
		belakang := ""

		as := string([]byte(temp)[0])
		kop := string([]byte(temp)[1])
		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])

		as_2, _ := strconv.Atoi(as)
		kop_2, _ := strconv.Atoi(kop)
		kepala_2, _ := strconv.Atoi(kepala)
		ekor_2, _ := strconv.Atoi(ekor)

		if as_2 <= 4 {
			data_1 = "KECIL"
		} else {
			data_1 = "BESAR"
		}
		if kop_2%2 == 0 {
			data_2 = "GENAP"
		} else {
			data_2 = "GANJIL"
		}
		if kepala_2 <= 4 {
			data_3 = "KECIL"
		} else {
			data_3 = "BESAR"
		}
		if ekor_2%2 == 0 {
			data_4 = "GENAP"
		} else {
			data_4 = "GANJIL"
		}

		depan = "DEPAN_" + data_1 + "_" + data_2
		tengah = "TENGAH_" + data_2 + "_" + data_3
		tengah2 = "TENGAH_" + data_3 + "_" + data_2
		belakang = "BELAKANG_" + data_3 + "_" + data_4

		if depan == nomorkeluaran {
			flag = true
		}
		if tengah == nomorkeluaran {
			flag = true
		}
		if tengah2 == nomorkeluaran {
			flag = true
		}
		if belakang == nomorkeluaran {
			flag = true
		}

		if flag {
			result = "WINNER"
		}
	case "DASAR":
		flag := false
		data_1 := ""
		data_2 := ""

		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])

		kepala_2, _ := strconv.Atoi(kepala)
		ekor_2, _ := strconv.Atoi(ekor)

		dasar := kepala_2 + ekor_2

		if dasar > 9 {
			temp2 := strconv.Itoa(dasar) //int to string
			temp21 := string([]byte(temp2)[0])
			temp22 := string([]byte(temp2)[1])

			temp21_2, _ := strconv.Atoi(temp21)
			temp22_2, _ := strconv.Atoi(temp22)
			dasar = temp21_2 + temp22_2
		}
		if dasar <= 4 {
			data_1 = "KECIL"
		} else {
			data_1 = "BESAR"
		}
		if dasar%2 == 0 {
			data_2 = "GENAP"
		} else {
			data_2 = "GANJIL"
		}

		if data_1 == nomorkeluaran {
			flag = true
		}
		if data_2 == nomorkeluaran {
			flag = true
		}

		if flag {
			result = "WINNER"
		}
	case "SHIO":
		flag := false

		kepala := string([]byte(temp)[2])
		ekor := string([]byte(temp)[3])
		data := _tableshio(kepala + ekor)

		if data == nomorkeluaran {
			flag = true
		}

		if flag {
			result = "WINNER"
		}
	}
	return result, win
}
func _tableshio(shiodata string) string {
	log.Printf("Shio : %s", shiodata)

	tglnow, _ := goment.New()
	yearnow := tglnow.Format("YYYY")
	log.Println(yearnow)
	result := ""
	switch yearnow {
	case "2022":
		harimau := []string{"01", "13", "25", "37", "49", "61", "73", "85", "97"}
		kerbau := []string{"02", "14", "26", "38", "50", "62", "74", "86", "98"}
		tikus := []string{"03", "15", "27", "39", "51", "63", "75", "87", "99"}
		babi := []string{"04", "16", "28", "40", "52", "64", "76", "88", "00"}
		anjing := []string{"05", "17", "29", "41", "53", "65", "77", "89", ""}
		ayam := []string{"06", "18", "30", "42", "54", "66", "78", "90", ""}
		monyet := []string{"07", "19", "31", "43", "55", "67", "79", "91", ""}
		kambing := []string{"08", "20", "32", "44", "56", "68", "80", "92", ""}
		kuda := []string{"09", "21", "33", "45", "57", "69", "81", "93", ""}
		ular := []string{"10", "22", "34", "46", "58", "70", "82", "94", ""}
		naga := []string{"11", "23", "35", "47", "59", "71", "83", "95", ""}
		kelinci := []string{"12", "24", "36", "48", "60", "72", "84", "96", ""}
		for i := 0; i < len(babi); i++ {
			if shiodata == babi[i] {
				result = "BABI"
			}
		}
		for i := 0; i < len(ular); i++ {
			if shiodata == ular[i] {
				result = "ULAR"
			}
		}
		for i := 0; i < len(anjing); i++ {
			if shiodata == anjing[i] {
				result = "ANJING"
			}
		}
		for i := 0; i < len(ayam); i++ {
			if shiodata == ayam[i] {
				result = "AYAM"
			}
		}
		for i := 0; i < len(monyet); i++ {
			if shiodata == monyet[i] {
				result = "MONYET"
			}
		}
		for i := 0; i < len(kambing); i++ {
			if shiodata == kambing[i] {
				result = "KAMBING"
			}
		}
		for i := 0; i < len(kuda); i++ {
			if shiodata == kuda[i] {
				result = "KUDA"
			}
		}
		for i := 0; i < len(naga); i++ {
			if shiodata == naga[i] {
				result = "NAGA"
			}
		}
		for i := 0; i < len(kelinci); i++ {
			if shiodata == kelinci[i] {
				result = "KELINCI"
			}
		}
		for i := 0; i < len(harimau); i++ {
			if shiodata == harimau[i] {
				result = "HARIMAU"
			}
		}
		for i := 0; i < len(kerbau); i++ {
			if shiodata == kerbau[i] {
				result = "KERBAU"
			}
		}
		for i := 0; i < len(tikus); i++ {
			if shiodata == tikus[i] {
				result = "TIKUS"
			}
		}
	case "2023":
		kelinci := []string{"01", "13", "25", "37", "49", "61", "73", "85", "97"}
		harimau := []string{"02", "14", "26", "38", "50", "62", "74", "86", "98"}
		kerbau := []string{"03", "15", "27", "39", "51", "63", "75", "87", "99"}
		tikus := []string{"04", "16", "28", "40", "52", "64", "76", "88", "00"}
		babi := []string{"05", "17", "29", "41", "53", "65", "77", "89", ""}
		anjing := []string{"06", "18", "30", "42", "54", "66", "78", "90", ""}
		ayam := []string{"07", "19", "31", "43", "55", "67", "79", "91", ""}
		monyet := []string{"08", "20", "32", "44", "56", "68", "80", "92", ""}
		kambing := []string{"09", "21", "33", "45", "57", "69", "81", "93", ""}
		kuda := []string{"10", "22", "34", "46", "58", "70", "82", "94", ""}
		ular := []string{"11", "23", "35", "47", "59", "71", "83", "95", ""}
		naga := []string{"12", "24", "36", "48", "60", "72", "84", "96", ""}
		for i := 0; i < len(babi); i++ {
			if shiodata == babi[i] {
				result = "BABI"
			}
		}
		for i := 0; i < len(ular); i++ {
			if shiodata == ular[i] {
				result = "ULAR"
			}
		}
		for i := 0; i < len(anjing); i++ {
			if shiodata == anjing[i] {
				result = "ANJING"
			}
		}
		for i := 0; i < len(ayam); i++ {
			if shiodata == ayam[i] {
				result = "AYAM"
			}
		}
		for i := 0; i < len(monyet); i++ {
			if shiodata == monyet[i] {
				result = "MONYET"
			}
		}
		for i := 0; i < len(kambing); i++ {
			if shiodata == kambing[i] {
				result = "KAMBING"
			}
		}
		for i := 0; i < len(kuda); i++ {
			if shiodata == kuda[i] {
				result = "KUDA"
			}
		}
		for i := 0; i < len(naga); i++ {
			if shiodata == naga[i] {
				result = "NAGA"
			}
		}
		for i := 0; i < len(kelinci); i++ {
			if shiodata == kelinci[i] {
				result = "KELINCI"
			}
		}
		for i := 0; i < len(harimau); i++ {
			if shiodata == harimau[i] {
				result = "HARIMAU"
			}
		}
		for i := 0; i < len(kerbau); i++ {
			if shiodata == kerbau[i] {
				result = "KERBAU"
			}
		}
		for i := 0; i < len(tikus); i++ {
			if shiodata == tikus[i] {
				result = "TIKUS"
			}
		}
	case "2024":
		naga := []string{"01", "13", "25", "37", "49", "61", "73", "85", "97"}
		kelinci := []string{"02", "14", "26", "38", "50", "62", "74", "86", "98"}
		harimau := []string{"03", "15", "27", "39", "51", "63", "75", "87", "99"}
		kerbau := []string{"04", "16", "28", "40", "52", "64", "76", "88", "00"}
		tikus := []string{"05", "17", "29", "41", "53", "65", "77", "89", ""}
		babi := []string{"06", "18", "30", "42", "54", "66", "78", "90", ""}
		anjing := []string{"07", "19", "31", "43", "55", "67", "79", "91", ""}
		ayam := []string{"08", "20", "32", "44", "56", "68", "80", "92", ""}
		monyet := []string{"09", "21", "33", "45", "57", "69", "81", "93", ""}
		kambing := []string{"10", "22", "34", "46", "58", "70", "82", "94", ""}
		kuda := []string{"11", "23", "35", "47", "59", "71", "83", "95", ""}
		ular := []string{"12", "24", "36", "48", "60", "72", "84", "96", ""}
		for i := 0; i < len(babi); i++ {
			if shiodata == babi[i] {
				result = "BABI"
			}
		}
		for i := 0; i < len(ular); i++ {
			if shiodata == ular[i] {
				result = "ULAR"
			}
		}
		for i := 0; i < len(anjing); i++ {
			if shiodata == anjing[i] {
				result = "ANJING"
			}
		}
		for i := 0; i < len(ayam); i++ {
			if shiodata == ayam[i] {
				result = "AYAM"
			}
		}
		for i := 0; i < len(monyet); i++ {
			if shiodata == monyet[i] {
				result = "MONYET"
			}
		}
		for i := 0; i < len(kambing); i++ {
			if shiodata == kambing[i] {
				result = "KAMBING"
			}
		}
		for i := 0; i < len(kuda); i++ {
			if shiodata == kuda[i] {
				result = "KUDA"
			}
		}
		for i := 0; i < len(naga); i++ {
			if shiodata == naga[i] {
				result = "NAGA"
			}
		}
		for i := 0; i < len(kelinci); i++ {
			if shiodata == kelinci[i] {
				result = "KELINCI"
			}
		}
		for i := 0; i < len(harimau); i++ {
			if shiodata == harimau[i] {
				result = "HARIMAU"
			}
		}
		for i := 0; i < len(kerbau); i++ {
			if shiodata == kerbau[i] {
				result = "KERBAU"
			}
		}
		for i := 0; i < len(tikus); i++ {
			if shiodata == tikus[i] {
				result = "TIKUS"
			}
		}
	}

	return result
}
func Pasaran_id(idcomppasaran int, company, tipecolumn string) (string, float32) {
	con := db.CreateCon()
	ctx := context.Background()
	var result string = ""
	var result_number float32 = 0
	sql_pasaran := `SELECT 
		idpasarantogel , 
		1_win4dbb, 1_win3dbb, 1_win3ddbb, 1_win2dbb, 1_win2ddbb, 1_win2dtbb, 
		2_win as win_cbebas, 3_win2digit as win2_cmacau, 
		3_win3digit as win3_cmacau, 3_win4digit as win4_cmacau, 
		4_win3digit as win3_cnaga, 4_win4digit as win4_cnaga 
		FROM ` + config.DB_tbl_mst_company_game_pasaran + `  
		WHERE idcomppasaran  = ? 
		AND idcompany = ? 
	`
	var (
		idpasarantogel_db                                                         string
		win4dbb_db, win3dbb_db, win3ddbb_db, win2dbb_db, win2ddbb_db, win2dtbb_db float32
		win_cbebas_db, win2_cmacau_db, win3_cmacau_db, win4_cmacau_db             float32
		win3_cnaga_db, win4_cnaga_db                                              float32
	)
	rows := con.QueryRowContext(ctx, sql_pasaran, idcomppasaran, company)
	switch err := rows.Scan(
		&idpasarantogel_db,
		&win4dbb_db, &win3dbb_db, &win3ddbb_db, &win2dbb_db, &win2ddbb_db, &win2dtbb_db,
		&win_cbebas_db, &win2_cmacau_db, &win3_cmacau_db, &win4_cmacau_db,
		&win3_cnaga_db, &win4_cnaga_db); err {
	case sql.ErrNoRows:
		result = ""
	case nil:
		switch tipecolumn {
		case "idpasarantogel":
			result = idpasarantogel_db
		case "1_win4dbb":
			result_number = win4dbb_db
		case "1_win3dbb":
			result_number = win3dbb_db
		case "1_win3ddbb":
			result_number = win3ddbb_db
		case "1_win2dbb":
			result_number = win2dbb_db
		case "1_win2ddbb":
			result_number = win2ddbb_db
		case "1_win2dtbb":
			result_number = win2dtbb_db
		case "2_win":
			result_number = win_cbebas_db
		case "3_win2digit":
			result_number = win2_cmacau_db
		case "3_win3digit":
			result_number = win3_cmacau_db
		case "3_win4digit":
			result_number = win4_cmacau_db
		case "4_win3digit":
			result_number = win3_cnaga_db
		case "4_win4digit":
			result_number = win4_cnaga_db
		}
	default:
		helpers.ErrorCheck(err)
	}
	return result, result_number
}
