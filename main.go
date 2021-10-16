package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/Go-SQL-Driver/MySQL"
)

// тип для пользователя
type TypeUsr struct {
	Id string `json:"id"`
}

// тип для активности приход/расход
type Activity struct {
	Id     string  `json:"id"`
	Amount float64 `json:"amount"`
}

type TypeTransfer struct {
	SenderId string  `json:"senderid"`
	Id       string  `json:"id"`
	Amount   float64 `json:"amount"`
}

// тип для возврата баланса
type BlAnswer struct {
	Id      string  `json:"id"`
	Balance float64 `json:"balance"`
}

//************** sql
// Обернем пул sql соединений
type dbType struct {
	DB *sql.DB
}

//-------------- Методы sql соединений

// Метод для создания движений
func (s dbType) AddAmount(qu Activity, op rune) (BlAnswer, error) {

	var noBalance bool     //пользователь есть/нет
	var newBalance float64 // новый баланс
	var desc string        // описание
	var bu BlAnswer        // возвращаемый баланс
	answ, err := s.BalanceFun(qu.Id)
	if err != nil {
		if err == sql.ErrNoRows { // если пользователь не найден
			noBalance = true
		} else {
			log.Fatal(err)
		}
	}
	if noBalance && op == 'a' {
		//создадим и вернем ошибку
		err1 := errors.New("user not found/balance cannot be negative")
		return bu, err1
	}

	switch op {
	case 'A':
		newBalance = float64(int(answ.Balance*100)+int(qu.Amount*100)) / 100
		desc = "поступление на баланс"
	case 'a':
		newBalance = float64(int(answ.Balance*100)-int(qu.Amount*100)) / 100
		desc = "списание с баланса"
	default:
		newBalance = -1
	}

	if newBalance < 0 {
		err1 := errors.New("balance cannot be negative")
		return bu, err1
	}

	var str1 string
	str2 := "INSERT transaction(user_id,transaction_date,transaction_type,amount,description) VALUES (?,?,?,?,?)"
	if noBalance {
		str1 = "INSERT users(user_id,date_create,balance) VALUES (?,?,?)"
	} else {
		str1 = "UPDATE users SET balance = ? WHERE user_id = ?"
	}

	tx, err := s.DB.Begin() // Транзакция начало
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	stmt1, err := tx.Prepare(str1)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt1.Close()
	dt := time.Now()
	if noBalance {
		_, err = stmt1.Exec(qu.Id, dt, newBalance)
	} else {
		_, err = stmt1.Exec(newBalance, qu.Id)
	}
	if err != nil {
		log.Fatal(err)
	}
	stmt2, err := tx.Prepare(str2)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt2.Close()
	_, err = stmt2.Exec(qu.Id, dt, string(op), qu.Amount, desc)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit() // Транзакция фиксируем
	if err != nil {
		log.Fatal(err)
	}

	bu.Id = qu.Id
	bu.Balance = newBalance

	return bu, err
}

// Метод для получения баланса
func (s dbType) BalanceFun(id string) (BlAnswer, error) {
	var baseBallance []uint8
	stmt, err := s.DB.Prepare("select user_id,balance from users where user_id = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	var bu BlAnswer
	err = stmt.QueryRow(id).Scan(&bu.Id, &baseBallance)
	if err != nil {
		if err == sql.ErrNoRows {
			return bu, err
		} else {
			log.Fatal(err)
		}
	}
	bu.Balance, err = strconv.ParseFloat(string(baseBallance), 64)
	if err != nil {
		log.Fatal(err)
	}
	return bu, nil
}

// Метод для  получения балансов множества пользователей
func (s dbType) balanceRows(id []interface{}) ([]BlAnswer, error) {
	var baseBallance []uint8
	str := "select user_id,balance from users where user_id in(?" + strings.Repeat(",?", len(id)-1) + ")"
	stmt, err := s.DB.Prepare(str)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var bu BlAnswer
	var buAnsw []BlAnswer

	rows, err := stmt.Query(id...)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&bu.Id, &baseBallance)
		if err != nil {
			log.Fatal(err)
		}
		bu.Balance, err = strconv.ParseFloat(string(baseBallance), 64)
		if err != nil {
			log.Fatal(err)
		}
		buAnsw = append(buAnsw, bu)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}

	return buAnsw, nil
}

// Метод для создания движений
func (s dbType) AddTransfer(qu TypeTransfer) ([]BlAnswer, error) {

	var newBalance, newBalanceSender float64 // новый баланс
	var desc string                          // описание
	var bu BlAnswer                          // возвращаемый баланс
	var buAnsw []BlAnswer

	users := []string{qu.SenderId, qu.Id}

	usersI := make([]interface{}, len(users))
	for i := range users {
		usersI[i] = users[i]
	}
	answ, err := s.balanceRows(usersI)
	if err != nil {
		log.Fatal(err)
	}

	if len(users) != len(answ) {
		// нет баланса пользователей
		err1 := errors.New("user not found")
		return buAnsw, err1
	}
	newBalanceSender = float64(int(answ[0].Balance*100)-int(qu.Amount*100)) / 100
	if newBalanceSender < 0 {
		err1 := errors.New("the balance of the sender cannot be negative")
		return buAnsw, err1
	}
	newBalance = float64(int(answ[0].Balance*100)+int(qu.Amount*100)) / 100

	str2 := "INSERT transaction(user_id,transaction_date,transaction_type,amount,description) VALUES (?,?,?,?,?)"
	str1 := "UPDATE users SET balance = ? WHERE user_id = ?"

	tx, err := s.DB.Begin() // Транзакция начало
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	stmt1, err := tx.Prepare(str1)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt1.Close()
	dt := time.Now()

	_, err = stmt1.Exec(newBalanceSender, qu.SenderId)
	if err != nil {
		log.Fatal(err)
	}
	_, err = stmt1.Exec(newBalance, qu.Id)
	if err != nil {
		log.Fatal(err)
	}
	stmt2, err := tx.Prepare(str2)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt2.Close()
	desc = fmt.Sprintf("перевод пользователю %s", qu.Id)
	_, err = stmt2.Exec(qu.SenderId, dt, string('a'), qu.Amount, desc)
	if err != nil {
		log.Fatal(err)
	}
	desc = fmt.Sprintf("перевод от пользователя %s", qu.SenderId)
	_, err = stmt2.Exec(qu.Id, dt, string('A'), qu.Amount, desc)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit() // Транзакция фиксируем
	if err != nil {
		log.Fatal(err)
	}

	bu.Id = qu.Id
	bu.Balance = newBalance
	buAnsw = append(buAnsw, bu)

	bu.Id = qu.SenderId
	bu.Balance = newBalanceSender
	buAnsw = append(buAnsw, bu)

	return buAnsw, err
}

//--------------
//**************

//************** http
type srvType struct {
	User interface {
		AddAmount(qu Activity, op rune) (BlAnswer, error)
		BalanceFun(id string) (BlAnswer, error)
		AddTransfer(qu TypeTransfer) ([]BlAnswer, error)
	}
}

//-------------- Методы http
func (srv *srvType) Tekst(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(rw, r)
		return
	}
	//rw.Header().Set("Content-type", "application/json; charset=utf-8")
	rw.Write([]byte("Server started " + time.Now().Format("01-02-2006 15:04:05")))
}

// Обработка запроса баланса
func (srv *srvType) GetBalance(rw http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, fmt.Sprintf("expect method POST /balance, got %v", r.Method), http.StatusMethodNotAllowed)
		return
	}

	var dj TypeUsr

	err := decodeJSONBody(rw, r, &dj)
	if err != nil {
		var mr *malformedRequest
		if errors.As(err, &mr) {
			http.Error(rw, mr.msg, mr.status)
		} else {
			log.Println(err.Error())
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	answ, err := srv.User.BalanceFun(dj.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			msg := fmt.Sprintf("User %s not founf", dj.Id)
			http.Error(rw, msg, http.StatusBadRequest)
			return
		} else {
			log.Fatal(err)
		}
	}
	js, err := json.Marshal(answ)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)
}

// Обработка трансфера
func (srv *srvType) Transfer(rw http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, fmt.Sprintf("expect method POST , got %v", r.Method), http.StatusMethodNotAllowed)
		return
	}

	var dj TypeTransfer

	err := decodeJSONBody(rw, r, &dj)
	if err != nil {
		var mr *malformedRequest
		if errors.As(err, &mr) {
			http.Error(rw, mr.msg, mr.status)
		} else {
			log.Println(err.Error())
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	answ, err := srv.User.AddTransfer(dj)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	js, err := json.Marshal(answ)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)

}

// Приход/расход по счету
func (srv *srvType) Ioflow(rw http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, fmt.Sprintf("expect method POST , got %v", r.Method), http.StatusMethodNotAllowed)
		return
	}

	var dj Activity
	var op rune // 'A' - приход  'a' - расход
	err := decodeJSONBody(rw, r, &dj)
	if err != nil {
		var mr *malformedRequest
		if errors.As(err, &mr) {
			http.Error(rw, mr.msg, mr.status)
		} else {
			log.Println(err.Error())
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	if r.URL.Path == "/inflow" {
		op = 'A'
	} else {
		op = 'a'
	}
	answ, err := srv.User.AddAmount(dj, op)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(answ)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)

}

//////////////////////////////////////////////

func main() {

	port := flag.String("port", ":6767", "port HTTP")
	dsn := flag.String("dsn", "user:1234@/mydb?charset=utf8&parseTime=true", "DSN MySQL ")
	flag.Parse()
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal("Mysql:", err)
	}
	defer db.Close()

	env := &srvType{
		User: dbType{DB: db},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", env.Tekst)
	mux.HandleFunc("/balance", env.GetBalance)
	mux.HandleFunc("/inflow", env.Ioflow)
	mux.HandleFunc("/outflow", env.Ioflow)
	mux.HandleFunc("/transfer", env.Transfer)
	log.Printf("Server started at %s %s", *port, time.Now().Format("01-02-2006 15:04:05"))
	er := http.ListenAndServe(*port, mux)
	log.Fatal("ListenAndServe:", er)

}
