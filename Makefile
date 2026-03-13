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

proto:
	rm -f pb/*.go
	protoc --proto_path=proto \
		--go_out=pb --go_opt=paths=source_relative \
		--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=pb --grpc-gateway_opt=paths=source_relative \
		proto/*.proto

.PHONY: postgres createdb dropdb migrateup migratedown proto