 
# microsvc

Учет баланса пользователя

## Ресурсы:
### `Поступление на счет/ Создание счета`
#### /inflow
Methods: POST
application/json
{"id": string[36], "amount": number}
### `Списание со счета`
#### /outflow 
Methods: POST
application/json
{"id": string[36], "amount": number}
### `Баланс счета`
#### /balance 
Methods: POST
application/json
{"id": string[36]}
### `Перевод между счетами`
#### /transfer 
Methods: POST
application/json
{"senderid": string[36], "id": string[36], "amount": number}


Все методы возвращают json c новым балансом счета/счетов : {"id": string[36], "amount": number}

## Пример запуска
microsvc -port ":6060" -dsn "user:1234@/mydb?charset=utf8&parseTime=true"

