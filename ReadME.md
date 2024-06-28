# Хранилище файлов с доступом по http

## Настройка перед запуском

```
go mod init main
go get -u github.com/gofiber/fiber/v2
go get -u github.com/gofiber/fiber/v2/middleware/logger
go get -u github.com/gofiber/fiber/v2/middleware/cors
```

## Запуск сервера

```
go run main
```

## Тестирование функционала

```
Upload:
curl -X POST http://localhost:1337/upload -F "file=@/path/to/your/file"
curl -X POST http://localhost:1337/upload -F "file=@/path/to/your/file" -F "md5=<md5_hash>" -F "sha1=<sha1_hash>" -F "sha256=<sha256_hash>"

Download:
curl -X GET http://localhost:1337/download/<file_hash> -o /path/to/save/file

Delete:
curl -X DELETE http://localhost:1337/delete/<file_hash>

```

## Выполненные допольнительные требования
* Добавить контроль целостности файлов: сервис не должен отдавать клиенту файл, если его локальная копия "побилась" (не совпадает хэш) клиент при загрузке (upload) может опционально указать один или несколько хешей (md5/sha1/sha256/...).
* Сервер должен проконтролировать, что хеши совпадают с реальным содержимым, и не сохранять файл (вернуть ошибку), если хотя бы один из хешей не совпадает.