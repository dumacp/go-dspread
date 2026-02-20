module github.com/dumacp/go-dspread

go 1.21.5

require (
	github.com/dumacp/smartCard v0.1.6
	github.com/dumacp/smartcard v0.0.0-00010101000000-000000000000
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
)

require (
	github.com/aead/cmac v0.0.0-20160719120800-7af84192f0b1 // indirect
	github.com/ebfe/scard v0.0.0-20230420082256-7db3f9b7c8a7 // indirect
	golang.org/x/sys v0.18.0 // indirect
)

replace github.com/dumacp/smartcard => ../smartcard
