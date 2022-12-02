package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buger/jsonparser"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/db"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/helpers"
	"github.com/nikitamirzani323/togel_apibackend_consumer_saveperiode/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

type generatorJobs struct {
	Idtrxkeluarandetail string
	Idtrxkeluaran       string
	Datetimedetail      string
	Company             string
	Username            string
	Nomortogel          string
	create              string
	createDate          string
}
type generatorResult struct {
	Idtrxkeluarandetail string
	Message             string
	Status              string
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
func main() {
	db.Init()
	AMPQ := os.Getenv("AMQP_SERVER_URL")
	conn, err := amqp.Dial(AMPQ)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"generator", // name
		false,       // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
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
			json := []byte(d.Body)
			jsonparser.ArrayEach(json, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
				Totaldata, _, _, _ := jsonparser.Get(value, "Totaldata")
				Idtrxkeluaran, _, _, _ := jsonparser.Get(value, "Idtrxkeluaran")
				Company, _, _, _ := jsonparser.Get(value, "Company")
				Record, _, _, _ := jsonparser.Get(value, "Record")

				total_record, _ := strconv.Atoi(string(Totaldata))
				// log.Printf("%s", total_record)
				// log.Println(string(Idtrxkeluaran))
				// log.Println(string(Company))
				flag := false
				tbl_trx_keluarantogel, tbl_trx_keluarantogel_detail, _ := models.Get_mappingdatabase(string(Company))
				// log.Println(string(Idtrxkeluaran))
				// log.Println(string(Company))
				// log.Println(tbl_trx_keluarantogel)
				flag = models.CheckDB(tbl_trx_keluarantogel, "idtrxkeluaran", string(Idtrxkeluaran))
				if flag {
					runtime.GOMAXPROCS(8)
					render_page := time.Now()
					totalWorker := 100
					total_data := total_record
					buffer_data := total_data + 1
					jobs_data := make(chan generatorJobs, buffer_data)
					results_data := make(chan generatorResult, buffer_data)

					wg := &sync.WaitGroup{}
					for w := 0; w < totalWorker; w++ {
						wg.Add(1)
						go _doJobInsertTransaksi(tbl_trx_keluarantogel_detail, jobs_data, results_data, wg)
					}

					jsonparser.ArrayEach(Record, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
						Idtrxkeluarandetail, _, _, _ := jsonparser.Get(value, "Idtrxkeluarandetail")
						Idtrxkeluaran, _, _, _ := jsonparser.Get(value, "Idtrxkeluaran")
						Datetimedetail, _, _, _ := jsonparser.Get(value, "Datetimedetail")
						Company, _, _, _ := jsonparser.Get(value, "Company")
						Username, _, _, _ := jsonparser.Get(value, "Username")
						Nomortogel, _, _, _ := jsonparser.Get(value, "Nomortogel")
						create, _, _, _ := jsonparser.Get(value, "create")
						createDate, _, _, _ := jsonparser.Get(value, "createDate")
						log.Printf("%s", Idtrxkeluarandetail)
						jobs_data <- generatorJobs{
							Idtrxkeluarandetail: string(Idtrxkeluarandetail),
							Idtrxkeluaran:       string(Idtrxkeluaran),
							Datetimedetail:      string(Datetimedetail),
							Company:             string(Company),
							Username:            string(Username),
							Nomortogel:          string(Nomortogel),
							create:              string(create),
							createDate:          string(createDate),
						}
					})
					close(jobs_data)
					for a := 0; a < total_data; a++ { //RESULT
						flag_result := <-results_data
						if flag_result.Status == "Failed" {
							log.Printf("ID : %s, Message: %s", flag_result.Idtrxkeluarandetail, flag_result.Message)
						}
					}
					close(results_data)
					wg.Wait()
					log.Println("Selesai Sudah")
					log.Println("TIME JOBS: ", time.Since(render_page).String())
					noinvoice, _ := strconv.Atoi(string(Idtrxkeluaran))
					_deleteredis_generator(string(Company), noinvoice)
				}

			})
		}
	}()

	log.Printf(" [*] Waiting for messages saveperiode. To exit press CTRL+C")
	<-forever
}
func _doJobInsertTransaksi(fieldtable string, jobs <-chan generatorJobs, results chan<- generatorResult, wg *sync.WaitGroup) {

	for capture := range jobs {
		for {
			var outerError error
			func(outerError *error) {
				defer func() {
					if err := recover(); err != nil {
						*outerError = fmt.Errorf("%v", err)
					}
				}()

				sql_insert := `
					insert into
					` + fieldtable + ` (
						idtrxkeluarandetail, idtrxkeluaran, datetimedetail,
						ipaddress, idcompany, username, typegame, nomortogel,posisitogel, bet,
						diskon, win, kei, browsertogel, devicetogel, statuskeluarandetail, 
						createkeluarandetail, createdatekeluarandetail
					) values (
						?, ?, ?, 
						?, ?, ?, ?, ?, ?,?, 
						?, ?, ?, ?, ?, ?,
						?, ?
					)
				`
				flag_insert, msg_insert := models.Exec_SQL(sql_insert, fieldtable, "INSERT",
					capture.Idtrxkeluarandetail, capture.Idtrxkeluaran, capture.Datetimedetail,
					"127.0.0.1", capture.Company, capture.Username, "4D", capture.Nomortogel, "FULL", 100, 0, 4000, 0,
					"ASIA/JAKARTA", "WEBSITE", "RUNNING",
					capture.create, capture.createDate)

				if !flag_insert {

					results <- generatorResult{Idtrxkeluarandetail: capture.Idtrxkeluarandetail, Message: msg_insert, Status: "Failed"}
				} else {
					results <- generatorResult{Idtrxkeluarandetail: capture.Idtrxkeluarandetail, Message: "Tidak ada masalah", Status: "Success"}
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
