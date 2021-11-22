module github.com/idena-network/idena-test-go

require (
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/go-stack/stack v1.8.1
	github.com/google/tink/go v0.0.0-20200401233402-a389e601043a
	github.com/idena-network/idena-go v0.28.1
	github.com/imdario/mergo v0.3.12
	github.com/ipfs/go-cid v0.0.7
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v0.0.0-20200227202807-02e2044944cc
	gopkg.in/urfave/cli.v1 v1.20.0
)

replace github.com/cosmos/iavl => github.com/idena-network/iavl v0.12.3-0.20210604085842-854e73deab29

replace github.com/libp2p/go-libp2p-pnet => github.com/idena-network/go-libp2p-pnet v0.2.1-0.20200406075059-75d9ee9b85ed

go 1.16
