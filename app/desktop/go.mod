module ai-platform/desktop

go 1.23

require (
	github.com/wailsapp/wails/v2 v2.12.0
	ai-platform/cmd/node v0.0.0
)

replace (
	ai-platform/cmd/node => ../../server/cmd/node
	ai-platform/pkg/translator => ../../server/pkg/translator
	ai-platform/pkg/provider => ../../server/pkg/provider
)
