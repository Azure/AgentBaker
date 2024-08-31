rm -r dist
GOOS=linux GOARCH=amd64 go build -o ./dist/installer-linux-amd64
GOOS=linux GOARCH=arm64 go build -o ./dist/installer-linux-arm64
GOOS=windows GOARCH=amd64 go build -o ./dist/installer-windows-amd64.exe
GOOS=windows GOARCH=arm64 go build -o ./dist/installer-windows-arm64.exe
