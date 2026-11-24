module marketmind-tests

go 1.27.3

require (
	github.com/jackc/pgx/v5 v5.12.0
	github.com/segmentio/kafka-go v0.4.51
	marketmind v0.0.0
)

replace marketmind => ../services

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/klauspost/compress v1.20.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	golang.org/x/text v0.43.0 // indirect
)
