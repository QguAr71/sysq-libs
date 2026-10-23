module github.com/QguAr71/sysq-libs

go 1.26.4

require golang.org/x/crypto v0.53.0

require (
	github.com/tggo/goSentencePiece v1.1.0 // indirect
	github.com/yalue/onnxruntime_go v1.31.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	github.com/QguAr71/sysq-core v0.0.0
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/QguAr71/sysq-core => /home/sysq/projects/sysq-core
