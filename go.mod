module github.com/idena-network/idena-test-go

go 1.13

replace github.com/tendermint/iavl => github.com/idena-network/iavl v0.12.3-0.20200414113415-041d1524315e

require (
	github.com/deckarep/golang-set v1.7.1
	github.com/go-stack/stack v1.8.0
	github.com/google/tink/go v0.0.0-20200401233402-a389e601043a
	github.com/idena-network/idena-go v0.21.3-0.20200720180944-61128177b9bc
	github.com/ipfs/go-cid v0.0.6
	github.com/pierrec/lz4 v2.5.2+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v0.0.0-20200227202807-02e2044944cc
	github.com/stretchr/testify v1.6.1
	gopkg.in/urfave/cli.v1 v1.20.0
)
