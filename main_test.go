package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

func TestInFlow(t *testing.T) {
	// Тестовые данные : уже созданный пользователю t1 с балансом 300 посылаем 700
	tBalance := 300.00
	tData := Activity{Id: "t1", Amount: 700}
	operation := "A"
	desc := "поступление на баланс"
	resBalance := 1000.00 // ожидаемое значение баланса

	json_data, err := json.Marshal(tData)
	if err != nil {
		log.Fatal(err)
	}

	db, mock, err := sqlmock.New()

	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	//баланс вернется в []unit8
	tBalanceArray := []byte(fmt.Sprintf("%.2f", tBalance))

	rows := sqlmock.NewRows([]string{"user_id", "balance"}).AddRow(tData.Id, tBalanceArray)
	mock.ExpectPrepare("select user_id,balance from users").ExpectQuery().WithArgs(tData.Id).WillReturnRows(rows)

	mock.ExpectBegin()
	mock.ExpectPrepare("UPDATE users").ExpectExec().WithArgs(tData.Amount+tBalance, tData.Id).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare("INSERT transaction").ExpectExec().WithArgs(tData.Id, AnyTime{}, operation, tData.Amount, desc).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	envTest := &srvType{
		User: dbType{DB: db},
	}

	r := http.NewServeMux()
	r.HandleFunc("/inflow", envTest.Ioflow)
	srv := httptest.NewServer(r)

	defer srv.Close()

	resp, err := http.Post(fmt.Sprintf("%s/inflow", srv.URL), "application/json",
		bytes.NewBuffer(json_data))

	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	//var res map[string]interface{}
	var res BlAnswer

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		t.Errorf("Error decoding %s", err.Error())
	}
	if res.Id == tData.Id {
		if res.Balance != resBalance {
			t.Errorf("balance =%f, want %f", res.Balance, resBalance)
		}
	} else {
		t.Errorf("id =%s, want %s", res.Id, tData.Id)
	}

}
