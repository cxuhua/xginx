module github.com/cxuhua/xginx

go 1.13

require (
	github.com/alibaba/sentinel-golang v0.4.0
	github.com/cxuhua/gopher-lua v1.0.1
	github.com/cxuhua/lzma v0.1.2
	github.com/functionalfoundry/graphqlws v0.0.0-20200611113535-7bc58903ce7b
	github.com/go-playground/validator/v10 v10.3.0
	github.com/gorilla/websocket v1.4.0
	github.com/graphql-go/graphql v0.7.9
	github.com/graphql-go/handler v0.2.3
	github.com/hashicorp/golang-lru v0.5.4
	github.com/json-iterator/go v1.1.10
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/mapstructure v1.3.3
	github.com/shopspring/decimal v1.2.0
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/x-cray/logrus-prefixed-formatter v0.5.2 // indirect
)

replace github.com/cxuhua/gopher-lua v1.0.1 => ../gopher-lua
