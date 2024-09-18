module github.com/dumacp/go-dspread

go 1.21.5

require (
	github.com/dumacp/smartcard v0.0.0-00010101000000-000000000000
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
)

require golang.org/x/sys v0.18.0 // indirect

replace github.com/dumacp/smartcard => ../smartcard
