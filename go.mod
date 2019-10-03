module github.com/idena-network/idena-test-go

go 1.12

replace github.com/tendermint/iavl => github.com/idena-network/iavl v0.12.3-0.20190919135148-89e4ad773677

require (
	github.com/RoaringBitmap/roaring v0.4.18
	github.com/go-stack/stack v1.8.0
	github.com/idena-network/idena-go v0.12.2
	github.com/stretchr/testify v1.4.0
	github.com/willf/bloom v2.0.3+incompatible
	gopkg.in/urfave/cli.v1 v1.20.0
)
