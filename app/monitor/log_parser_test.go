package monitor

import (
	"context"
	"strings"
	"testing"
)

var lines = `[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:   export GIN_MODE=release
 - using code:  gin.SetMode(gin.ReleaseMode)
{"L":"INFO","T":"2024-07-02T05:56:54.617Z","C":"bootstrap/http.go:67","M":"Mux loaded successfully","TraceID":"ea00fd99a245-d2etln81kvqj-5yc1t"}
[GIN-debug] GET    /go-api/internal/ping     --> github.com/seakee/go-api/app/http/router.internal.func1 (5 handlers)
[GIN-debug] GET    /go-api/internal/admin/ping --> github.com/seakee/go-api/app/http/router.internal.func2 (5 handlers)
[GIN-debug] GET    /go-api/internal/service/ping --> github.com/seakee/go-api/app/http/router.internal.func3 (5 handlers)
[GIN-debug] POST   /go-api/internal/service/server/auth/app --> github.com/seakee/go-api/app/http/controller/auth.handler.Create.func1 (6 handlers)
[GIN-debug] POST   /go-api/internal/service/server/auth/token --> github.com/seakee/go-api/app/http/controller/auth.handler.GetToken.func1 (5 handlers)
[GIN-debug] GET    /go-api/external/ping     --> github.com/seakee/go-api/app/http/router.external.func1 (5 handlers)
[GIN-debug] GET    /go-api/external/app/ping --> github.com/seakee/go-api/app/http/router.external.func2 (5 handlers)
[GIN-debug] GET    /go-api/external/service/ping --> github.com/seakee/go-api/app/http/router.external.func3 (5 handlers)

panic: eee [recovered]
	panic: eee

goroutine 9 [running]:
testing.tRunner.func1.2({0x7a70020, 0x7b29c08})
	/WorkSpace/Golang/go1.22.4/src/testing/testing.go:1631 +0x49e
testing.tRunner.func1()
	/WorkSpace/Golang/go1.22.4/src/testing/testing.go:1634 +0x669
panic({0x7a70020?, 0x7b29c08?})
	/WorkSpace/Golang/go1.22.4/src/runtime/panic.go:770 +0x136
github.com/seakee/dockmon/app/monitor.(*handler).processUnstructuredLog(0xc000211180, {0x7b2f5f8, 0x7e6a8a0}, {0x79a4785, 0x6})
	/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser.go:91 +0x355
github.com/seakee/dockmon/app/monitor.(*handler).processLogLine(0xc000211180, {0x7b2f5f8, 0x7e6a8a0}, {0x79caf73, 0x114}, {0x79a4785, 0x6}, {0x79a57d4, 0x8})
	/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser.go:125 +0x32f
github.com/seakee/dockmon/app/monitor.TestProcessLogLine(0xc0000aaea0)
	/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser_test.go:44 +0x40f
testing.tRunner(0xc0000aaea0, 0x7b27720)
	/WorkSpace/Golang/go1.22.4/src/testing/testing.go:1689 +0x1da
created by testing.(*T).Run in goroutine 1
	/WorkSpace/Golang/go1.22.4/src/testing/testing.go:1742 +0x7d3
2024/07/01 09:50:04 /build/app/model/qinglong/cron.go:19 record not found
{"L":"ERROR","T":"2024-07-02T15:00:27.978+0800","C":"monitor/log_parser.go:70","M":"create log error","TraceID":"ea00fd99a245.lan-d2euy7h89o6g-5yc1v","error":"create err: Error 1292 (22007): Incorrect datetime value: '0000-00-00' for column 'time' at row 1","errorVerbose":"Error 1292 (22007): Incorrect datetime value: '0000-00-00' for column 'time' at row 1\ncreate err\ngithub.com/seakee/dockmon/app/model/monitor.(*Log).Create\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/model/monitor/log.go:48\ngithub.com/seakee/dockmon/app/repository/monitor.(*repo).CreateLog\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/repository/monitor/log.go:22\ngithub.com/seakee/dockmon/app/service/monitor.logService.Store\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/service/monitor/log.go:25\ngithub.com/seakee/dockmon/app/monitor.(*handler).storeLog\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser.go:69\ngithub.com/seakee/dockmon/app/monitor.(*handler).processUnstructuredLog\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser.go:87\ngithub.com/seakee/dockmon/app/monitor.(*handler).processLogLine\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/log_parser.go:115\ngithub.com/seakee/dockmon/app/monitor.(*handler).collectLogs\n\t/WorkSpace/Golang/src/github.com/seakee/dockmon/app/monitor/handler.go:205\nruntime.goexit\n\t/WorkSpace/Golang/go1.22.4/src/runtime/asm_amd64.s:1695"}
`

func TestProcessLogLine(t *testing.T) {
	h, err := newTestCollector()
	if err != nil {
		t.Error(err)
	}

	h.unstructuredLogs.entries["testID"] = &unstructuredLogBuffer{
		containerID:   "testID",
		containerName: "testName",
		logs:          make([]string, 0),
	}

	sections := strings.Split(lines, "\n")
	for _, section := range sections {
		h.processLogLine(context.Background(), "2024-07-02T16:53:00.265+0800", section, "testID", "testName")
	}
}
