export GO111MODULE=on
#export GOPROXY=https://goproxy.io
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig

echo "fmt"
gofmt -s -w .

go mod tidy
go build
