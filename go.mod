module github.com/idena-network/idena-test-go

go 1.12

replace github.com/tendermint/iavl => github.com/idena-network/iavl v0.0.0-20190701090235-eef65d855b4a

require (
	github.com/go-stack/stack v1.8.0
	github.com/idena-network/idena-go v0.5.5
	github.com/stretchr/testify v1.3.0
	gopkg.in/urfave/cli.v1 v1.20.0
)
