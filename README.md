# Distributed-calculation
Project from Yandex Lyceum Course - Programming in Go
Start point: http://localhost:8080

## Как запустить: 
0) Установите [Docker engine](https://docs.docker.com/engine/install/) и [Docker compose](https://docs.docker.com/compose/install/)
1) `cd <"path/to/project/root/directory">`
3) Установите пароли для redis и postgres в файле  `.env.example`
4) Переименуйте `.env.example` в `.env` 
5) `docker-compose -f docker-compose.yml up  -d`

## Описание запросов:
### Регистрация
POST `http://localhost:8080/register`
#### Тело запроса:
  ```json
{
    "login": "klef99",
    "password": "123abCC"
}
```
#### Тело ответа:
OK, если регистрация успешна. Иначе http код не 200.

### Авторизация
POST `http://localhost:8080/login`
#### Тело запроса:
  ```json
{
    "login": "klef99",
    "password": "123abCC"
}
```
#### Тело ответа:
JWT токет, если регистрация успешна. Иначе http код не 200.
### Получить статус агентов:
GET `http://localhost:8080/getWorkersStatus`
#### Тело ответа:
```json
[
    {
        "workerName": "worker1",
        "status": "OK",
        "taskCount": "2"
    }
]
```
Агент признан недоступным, если с момента последнего hearthbeat прошла минута. Если недоступно ни одного агента, то выражения не отправляюся на расчёт. taskCount - количество рассчитываемых на данный момент операций. Так как это метод для внутреннего пользования, он не требует токен.
## Следующие методы требуют наличия jwt токена в Headers
Вид следующий: Authorization: Bearer \<token>
### Отправить выражение:
POST `http://localhost:8080/addExpression`
В этом запросе пользователь может отправить свой id как ключ идемпотентности. Если header:X-Request-Id - пустой, в теле ответа возращается сгенерированный сервером uuid. Иначе используется uuid пользователся. Для создания uuid можно пользоваться этим [сайтом](https://www.uuidgenerator.net/).
#### Тело запроса:
  ```json
  {
      "expression": "2+2/1+2/1"
  }
  ```
#### Тело ответа:
```json
{
    "expressionid": "603b53cb-2175-46bd-a15f-bfba1e1918fb",
    "expression": "2+2/1+2/1",
    "status": 0
}
```
### Получить статус выражения по id:
GET `http://localhost:8080/getExpressionByID?expressionId=<expressionid>`
#### Тело ответа:
```json
{
    "expressionId": "603b53cb-2175-46bd-a15f-bfba1e1918fb",
    "status": 2,
    "result": 6
}
```
#### Значения статус-кодов выражений:
1. 0 - Выражение было добавлено в бд.
2. 1 - Выражение было разделено на элементарные операции.
3. 2 - Выражение было посчитано (result != null)
4. -1 - Выражение было признано невалидным при вычислении.

### Получить все выражения в бд:
GET `http://localhost:8080/getExpressionsList`
#### Тело ответа:
```json
[
    {
        "expressionid": "edd8d169-7e60-41ea-8d3c-e8766718461a",
        "expression": "(1+1))",
        "status": -1,
        "result": null
    },
    {
        "expressionid": "d4be595a-f538-4132-a14b-efe7784d5aa5",
        "expression": "((5*3)+(8/2)-(7*4)/(6-3)*(9+1)/(2*5)-(6/2)+(3*2)+(4-1)/(9*1)*(2+7)/(8-6)*(5/5))",
        "status": 2,
        "result": 14.166666666666666
    },
    {
        "expressionid": "603b53cb-2175-46bd-a15f-bfba1e1918fb",
        "expression": "2+2/1+2/1",
        "status": 2,
        "result": 6
    }
]
```
### Установить время расчёта одной операции:
POST `http://localhost:8080/setOperationsTimeout`
В этом запросе 
#### Тело запроса:
  ```json
{
    "*": 3,
    "+": 5,
    "-": 5,
    "/": 10
}
  ```
Тело может содержать произвольное количество операций (от 0 до 4). Если данных о какой-либо операции нет в redis, то для этой операции ставится значение по умолчанию (10 секунд). Таймаут в секундах.
#### Тело ответа:
```
OK
```
### Получить время расчёта одной операции:
GET `http://localhost:8080/getOperationsTimeout`
В этом запросе 
#### Тело ответа:
  ```json
{
    "*": 3,
    "+": 5,
    "-": 5,
    "/": 10
}
  ```

## Спецификации:
1. [Критерии](/docs/criteria.md)
## Нет frontend

Для более удобного ознакомления с системой, следует использовать [postman](https://www.postman.com/downloads/)
[Postman file](docs/Distibuted%20calculation.postman_collection.json).

# Общая схема системы
![image](docs/system%20scheme.svg)
# Схема базы данных
![image](docs/database%20struct.svg)