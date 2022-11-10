rm -rf bin
mkdir bin

export GOOS=linux
export GOARCH=amd64
go build -o bin/plotbot
