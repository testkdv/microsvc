 
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



