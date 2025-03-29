serve:
  go run server.go

compile:
  GOOS=js GOARCH=wasm go build -o docs/main.wasm main.go