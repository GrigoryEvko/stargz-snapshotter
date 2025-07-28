module github.com/containerd/stargz-snapshotter/estargz

go 1.24.0

toolchain go1.24.5

require (
	github.com/GrigoryEvko/gozstd v1.22.1
	github.com/containerd/log v0.1.0
	github.com/containerd/stargz-snapshotter v0.0.0-00010101000000-000000000000
	github.com/klauspost/compress v1.18.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/vbatts/tar-split v0.12.1
	golang.org/x/sync v0.16.0
)

require (
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shirou/gopsutil/v4 v4.25.6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/gozstd v1.22.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sys v0.34.0 // indirect
)

replace github.com/containerd/stargz-snapshotter => github.com/GrigoryEvko/stargz-snapshotter v0.17.1
