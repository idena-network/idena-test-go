module github.com/idena-network/idena-test-go

go 1.13

replace github.com/cosmos/iavl => github.com/idena-network/iavl v0.12.3-0.20210112075003-70ccb13c86c9

require (
	github.com/deckarep/golang-set v1.7.1
	github.com/go-stack/stack v1.8.0
	github.com/google/tink/go v0.0.0-20200401233402-a389e601043a
	github.com/idena-network/idena-go v0.24.3
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v0.0.0-20200227202807-02e2044944cc
	github.com/stretchr/testify v1.7.0
	gopkg.in/urfave/cli.v1 v1.20.0
)
