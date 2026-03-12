DB_URL=postgresql://root:secret@localhost:5432/escrow_db?sslmode=disable

postgres:
	docker run --name escrow-postgres -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:14-alpine

createdb:
	docker exec -it escrow-postgres createdb --username=root --owner=root escrow_db

dropdb:
	docker exec -it escrow-postgres dropdb escrow_db

migrateup:
	migrate -path db/migration -database "$(DB_URL)" -verbose up

migratedown:
	migrate -path db/migration -database "$(DB_URL)" -verbose down

.PHONY: postgres createdb dropdb migrateup migratedown