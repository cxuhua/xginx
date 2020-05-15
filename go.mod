module github.com/cxuhua/xginx

go 1.13

require (
	github.com/cxuhua/gopher-lua v1.0.1
	github.com/json-iterator/go v1.1.9
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/goleveldb v1.0.0
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

replace github.com/cxuhua/gopher-lua v1.0.1 => ../gopher-lua
