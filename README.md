# Блог 
Блог, написанный на Go и использующий БД PostgreSQ. Реализует следующие функции:
1. Список статей (главная страница)
2. Регистрация автора
3. Добавление статьи (только для зарегистрированных авторов)
4. Отдельный эндпоинт для метрик приложения (/metrics)

## Требования к серверу
1. Установлен golang версии не ниже 1.24
2. Установлен postgresql версии не ниже 16
3. Подоготовка БД:
```sql
create user blogadmin with password 'secretpassword';
create database blog owner blogadmin;
```

## Запуск приложения
Запустить можно напрямую командой 
```bash
$ go run main.go
```
Или же скомпилировать бинарный файл
```bash
$ go build -o go-blog main.go
$ ./go-blog
```

## Проверки
```
# регистрация автора
curl -X POST -H "Content-Type: application/json" -d @reg.json 127.0.0.1:8080/register
# проверка (total_authors должно быть 1)
curl 127.0.0.1:8080/metrics

# добавление статьи
curl -u testuser:securepassword -X POST -H "Content-Type: application/json" -d @article.json 127.0.0.1:8080/articles
# проверка (total_articles должно быть 1)
curl 127.0.0.1:8080/metrics 
```
