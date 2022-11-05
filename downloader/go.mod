module downloader

go 1.19

require (
	github.com/go-git/go-git/v5 v5.4.2
	github.com/vbauerster/mpb/v8 v8.1.4
)

require (
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210428141323-04723f9f07d7 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20210326060303-6b1517762897 // indirect
	golang.org/x/sys v0.1.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

// TODO: Waiting for https://github.com/go-git/go-git/pull/485 to be merged
replace github.com/go-git/go-git/v5 => github.com/ZauberNerd/go-git/v5 v5.4.3-0.20220315170230-29ec1bc1e5db
