GOOS=linux GOARCH=amd64 go build -o ./dist/node-bootstrapper-linux-amd64
GOOS=linux GOARCH=arm64 go build -o ./dist/node-bootstrapper-linux-arm64
GOOS=windows GOARCH=amd64 go build -o ./dist/node-bootstrapper-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build -o ./dist/node-bootstrapper-windows-arm64.exe
